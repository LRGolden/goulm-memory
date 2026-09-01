package memory

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Constantes de persistencia.
const (
	lockStaleAfter    = 15 * time.Second
	lockWaitMax       = 10 * time.Second
	lockRefreshEvery  = 5 * time.Second // heartbeat: refrescar lock mientras se persiste
	lockMaxClockSkew  = 5 * time.Second // tolerancia de reloj hacia el futuro
	defaultMaxEntries = 100
	defaultMaxBackups = 10
	maxNearDupPairs   = 500   // limite de comparaciones Jaccard en Consolidate
	maxMaxEntries     = 10000 // cap duro para MaxEntries
	maxMaxBackups     = 100   // cap duro para MaxBackups
	maxArchiveEntries = 500   // limite de capsulas en archive
)

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

// generateLockID genera un identificador unico para el lock (UUID v4).
func generateLockID() string {
	var buf [16]byte
	cryptorand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

// cleanupStaleTmp elimina archivos temporales huérfanos de crashes anteriores.
func cleanupStaleTmp(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// jsonMarshalIndent serializa con indentación de 2 espacios.
func jsonMarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// Config configura un MemoryStore.
type Config struct {
	Dir        string // ~/.goulm/memory/<proyecto-id>
	Format     Format // json (default) | ambar
	Project    string // nombre declarado en los archivos
	MaxEntries int    // límite de cápsulas activas (default 100)
	MaxBackups int    // backups a conservar (default 10)
	MaxArchive int    // límite de cápsulas en archive (default 500)
}

// FileSet describe los archivos del almacén para el formato actual.
type FileSet struct {
	Memory   string
	Archive  string
	Config   string
	Lock     string
	Backups  string
	Sessions string
}

// fileSet construye los nombres de archivo según el formato.
func fileSet(dir string, f Format) FileSet {
	ext := ".json"
	if f == FormatAmbar {
		ext = ".amb"
	}
	return FileSet{
		Memory:   filepath.Join(dir, "memory"+ext),
		Archive:  filepath.Join(dir, "archive"+ext),
		Config:   filepath.Join(dir, "config.json"),
		Lock:     filepath.Join(dir, "memory.lock"),
		Backups:  filepath.Join(dir, "backups"),
		Sessions: filepath.Join(dir, "sessions"),
	}
}

// storeConfig es el archivo config.json del almacén.
type storeConfig struct {
	Format     Format              `json:"format"`
	Project    string              `json:"project"`
	MaxEntries int                 `json:"max_entries"`
	MaxBackups int                 `json:"max_backups,omitempty"`
	MaxArchive int                 `json:"max_archive,omitempty"`
	Vocab      map[string][]string `json:"vocab,omitempty"`
}

// fileModel es el esquema de archivo (json o ambar).
type fileModel struct {
	Version  int        `json:"version"`
	Project  string     `json:"project"`
	Updated  string     `json:"updated"`
	Capsules []*Capsule `json:"capsules"`
}

// fileStamp es la marca de modtime+size de un archivo persistido.
type fileStamp struct {
	mod  int64
	size int64
}

func stampOf(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: fi.ModTime().UnixNano(), size: fi.Size()}
}

// MemoryStore es el almacén de memoria de un proyecto.
type MemoryStore struct {
	mu       sync.Mutex
	cfg      Config
	files    FileSet
	entries  map[string]*Capsule // por ID
	archive  map[string]*Capsule // por ID
	vocab    map[string][]string
	byKeyIdx map[string]string // key → ID (índice de búsqueda por clave)
	embedder EmbeddingProvider

	memStamp       fileStamp // marca del memory file en la última carga/persist
	dirty          bool      // bumps pendientes de persistir (Flush)
	graphVer       int       // versión de mutación para invalidar la cache de grafo
	cachedGraph    *Graph
	cachedCentral  map[string]float64
	cachedGraphDay string // YYYY-MM-DD (la visibilidad cambia por día)
	cachedAsOf     string
	cachedGraphVer int
	cachedVP       *VPTree // VP-Tree cacheado para vector search
	cachedVPVer    int     // versión de mutación del VP-Tree
}

// NewStore crea (o abre) el almacén del proyecto. Si la carpeta no existe,
// la crea junto con config.json.
func NewStore(cfg Config) (*MemoryStore, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("Config.Dir es obligatorio")
	}
	if cfg.Format == "" {
		cfg.Format = FormatJSON
	}
	if cfg.Format != FormatJSON && cfg.Format != FormatAmbar {
		return nil, fmt.Errorf("formato inválido: %q (json o ambar)", cfg.Format)
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultMaxEntries
	}
	if cfg.MaxEntries > maxMaxEntries {
		cfg.MaxEntries = maxMaxEntries
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = defaultMaxBackups
	}
	if cfg.MaxBackups > maxMaxBackups {
		cfg.MaxBackups = maxMaxBackups
	}
	if cfg.MaxArchive <= 0 {
		cfg.MaxArchive = maxArchiveEntries
	}
	if cfg.MaxArchive > maxArchiveEntries {
		cfg.MaxArchive = maxArchiveEntries
	}
	if err := os.MkdirAll(cfg.Dir, 0700); err != nil {
		return nil, fmt.Errorf("creando directorio de memoria: %w", err)
	}
	cleanupStaleTmp(cfg.Dir)
	s := &MemoryStore{
		cfg:      cfg,
		files:    fileSet(cfg.Dir, cfg.Format),
		entries:  make(map[string]*Capsule),
		archive:  make(map[string]*Capsule),
		byKeyIdx: make(map[string]string),
	}
	if err := s.loadMeta(); err != nil {
		return nil, err
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	s.memStamp = stampOf(s.files.Memory)
	return s, nil
}

// SetFormat cambia el formato de almacenamiento y reescribe el archivo.
// Los datos se conservan (roundtrip a través del modelo en memoria).
func (s *MemoryStore) SetFormat(f Format) error {
	if f != FormatJSON && f != FormatAmbar {
		return fmt.Errorf("formato inválido: %q", f)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Format = f
	s.files = fileSet(s.cfg.Dir, f)
	return s.persistLocked()
}

// loadMeta lee config.json (formato, proyecto, límites, vocabulario).
func (s *MemoryStore) loadMeta() error {
	sc := storeConfig{Format: s.cfg.Format, Project: s.cfg.Project, MaxEntries: s.cfg.MaxEntries}
	data, err := os.ReadFile(s.files.Config)
	if err == nil {
		if e := json.Unmarshal(data, &sc); e != nil {
			return fmt.Errorf("config.json corrupto: %w", e)
		}
	}
	s.cfg.Format = sc.Format
	if sc.Project != "" {
		s.cfg.Project = sc.Project
	}
	if sc.MaxEntries > 0 && sc.MaxEntries <= maxMaxEntries {
		s.cfg.MaxEntries = sc.MaxEntries
	}
	if sc.MaxBackups > 0 && sc.MaxBackups <= maxMaxBackups {
		s.cfg.MaxBackups = sc.MaxBackups
	}
	if sc.MaxArchive > 0 && sc.MaxArchive <= maxArchiveEntries {
		s.cfg.MaxArchive = sc.MaxArchive
	}
	s.vocab = sc.Vocab
	if s.vocab == nil {
		s.vocab = make(map[string][]string)
	}
	s.files = fileSet(s.cfg.Dir, s.cfg.Format)
	return nil
}

// load lee memory + archive desde disco. Si un archivo está corrupto,
// lo renombra como backup y continúa con estado vacío (recovery mode).
func (s *MemoryStore) load() error {
	if err := s.loadFile(s.files.Memory, s.entries, true); err != nil {
		s.backupCorruptFile(s.files.Memory)
	}
	if err := s.loadFile(s.files.Archive, s.archive, false); err != nil {
		s.backupCorruptFile(s.files.Archive)
	}
	return nil
}

// backupCorruptFile renombra un archivo corrupto como .corrupt.<timestamp>
// para permitir que el store se abra con estado parcial.
func (s *MemoryStore) backupCorruptFile(path string) {
	ts := time.Now().Format("20060102-150405")
	backup := path + ".corrupt." + ts
	os.Rename(path, backup)
}

func (s *MemoryStore) loadFile(path string, target map[string]*Capsule, indexKeys bool) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("leyendo %s: %w", filepath.Base(path), err)
	}
	caps, err := s.decode(data)
	if err != nil {
		return fmt.Errorf("%s corrupto: %w", filepath.Base(path), err)
	}
	for _, c := range caps {
		target[c.ID] = c
		if indexKeys {
			s.byKeyIdx[c.Key] = c.ID
		}
	}
	return nil
}

func (s *MemoryStore) decode(data []byte) ([]*Capsule, error) {
	if s.cfg.Format == FormatAmbar {
		_, caps, err := UnmarshalAmbar(string(data))
		return caps, err
	}
	var m fileModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m.Capsules, nil
}

func (s *MemoryStore) encode(capsules []*Capsule) []byte {
	if s.cfg.Format == FormatAmbar {
		return []byte(MarshalAmbar(s.cfg.Project, capsules))
	}
	m := fileModel{Version: 1, Project: s.cfg.Project, Updated: nowISO(), Capsules: capsules}
	out, _ := json.MarshalIndent(m, "", "  ")
	return out
}

// persistLocked escribe memory + archive + config de forma atómica, bajo el
// lockfile de proceso (multi-proceso) y tras adoptar cambios ajenos en disco.
// El llamador debe mantener s.mu.
func (s *MemoryStore) persistLocked() error {
	release, lockID, err := lockFile(s.files.Lock)
	if err != nil {
		return err
	}
	defer release()

	// Heartbeat: refrescar timestamp antes de operaciones largas para que
	// otros procesos no reclamen el lock como stale mientras trabajamos.
	refreshLock(s.files.Lock, lockID)
	s.adoptForeignLocked()
	refreshLock(s.files.Lock, lockID)
	if err := s.writeLocked(); err != nil {
		return err
	}
	s.memStamp = stampOf(s.files.Memory)
	return s.writeMetaLocked()
}

// adoptForeignLocked fusiona cápsulas que otros procesos escribieron en disco
// desde nuestra última carga (claves desconocidas para nosotros). Evita la
// pérdida de memoria entre procesos concurrentes (last-writer-wins).
func (s *MemoryStore) adoptForeignLocked() {
	if stampOf(s.files.Memory) == s.memStamp {
		return
	}
	data, err := os.ReadFile(s.files.Memory)
	if err != nil {
		return
	}
	caps, err := s.decode(data)
	if err != nil {
		return
	}
	// Dedupe por ID y por clave en ambos mapas (entries está indexado por ID;
	// la clave es la identidad lógica de la cápsula).
	keyExists := func(key string) bool {
		if _, ok := s.byKey(key); ok {
			return true
		}
		for _, a := range s.archive {
			if a.Key == key {
				return true
			}
		}
		return false
	}
	changed := false
	for _, c := range caps {
		if c == nil || c.Key == "" {
			continue
		}
		if _, ok := s.entries[c.ID]; ok {
			continue
		}
		if keyExists(c.Key) {
			continue
		}
		s.entries[c.ID] = c
		s.byKeyIdx[c.Key] = c.ID
		changed = true
	}
	if data, err := os.ReadFile(s.files.Archive); err == nil {
		if acaps, err := s.decode(data); err == nil {
			for _, c := range acaps {
				if c == nil || c.Key == "" {
					continue
				}
				if _, ok := s.entries[c.ID]; ok {
					continue
				}
				if _, ok := s.archive[c.ID]; ok {
					continue
				}
				if keyExists(c.Key) {
					continue
				}
				s.archive[c.ID] = c
				changed = true
			}
		}
	}
	if changed {
		s.bumpGraph()
	}
}

func (s *MemoryStore) writeLocked() error {
	active := make([]*Capsule, 0, len(s.entries))
	for _, c := range s.entries {
		active = append(active, c)
	}
	slices.SortStableFunc(active, func(a, b *Capsule) int {
		return strings.Compare(a.Key, b.Key)
	})
	if err := atomicWrite(s.files.Memory, s.encode(active), 0600); err != nil {
		return err
	}
	arch := make([]*Capsule, 0, len(s.archive))
	for _, c := range s.archive {
		arch = append(arch, c)
	}
	slices.SortStableFunc(arch, func(a, b *Capsule) int {
		return strings.Compare(a.Key, b.Key)
	})
	return atomicWrite(s.files.Archive, s.encode(arch), 0600)
}

func (s *MemoryStore) writeMetaLocked() error {
	sc := storeConfig{
		Format:     s.cfg.Format,
		Project:    s.cfg.Project,
		MaxEntries: s.cfg.MaxEntries,
		Vocab:      s.vocab,
	}
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.files.Config, data, 0600)
}

// Flush persiste los bumps de acceso pendientes (dirty). Es el punto de
// escritura diferido que evita fsync por recall.
func (s *MemoryStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	if err := s.persistLocked(); err != nil {
		return err
	}
	s.dirty = false
	return nil
}

// bumpGraph invalida la cache de grafo y VP-Tree tras una mutación del almacén.
func (s *MemoryStore) bumpGraph() { s.graphVer++ }

// graphFor devuelve el grafo del proyecto cacheado por (versión, día, asOf).
// La visibilidad depende de la fecha (TTL) y asOf, por eso entran en la clave.
func (s *MemoryStore) graphFor(now time.Time, asOf string) *Graph {
	day := now.Format("2006-01-02")
	if s.cachedGraph != nil && s.cachedGraphDay == day && s.cachedAsOf == asOf && s.cachedGraphVer == s.graphVer {
		return s.cachedGraph
	}
	all := make([]*Capsule, 0, len(s.entries))
	for _, c := range s.entries {
		if c.IsVisible(now, asOf) {
			all = append(all, c)
		}
	}
	g := BuildGraph(all)
	s.cachedGraph = g
	s.cachedCentral = g.Centrality()
	s.cachedGraphDay = day
	s.cachedAsOf = asOf
	s.cachedGraphVer = s.graphVer
	return g
}

// centralityFor devuelve la centralidad cacheada del grafo actual.
func (s *MemoryStore) centralityFor(now time.Time, asOf string) map[string]float64 {
	s.graphFor(now, asOf)
	return s.cachedCentral
}

// vpTreeFor devuelve el VP-Tree cacheado, reconstruyéndolo si la versión
// de mutación ha cambiado. El tree se construye sobre todas las cápsulas
// visibles con embeddings.
func (s *MemoryStore) vpTreeFor(now time.Time, asOf string) *VPTree {
	if s.cachedVP != nil && s.cachedVPVer == s.graphVer {
		return s.cachedVP
	}
	// Recopilar cápsulas visibles con embeddings.
	all := make([]*Capsule, 0, len(s.entries))
	for _, c := range s.entries {
		if c.IsVisible(now, asOf) && len(c.Embedding) > 0 {
			all = append(all, c)
		}
	}
	s.cachedVP = BuildVPTree(all)
	s.cachedVPVer = s.graphVer
	return s.cachedVP
}

// atomicWrite escribe un archivo de forma atómica: temp + rename.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// lockFile adquiere el lockfile de proceso (PID + antigüedad + UUID).
// Devuelve una función de liberación y el UUID del lock adquirido.
// Busy-wait con timeout y jitter anti-thundering-herd.
func lockFile(lockPath string) (func(), string, error) {
	pid := os.Getpid()
	myUUID := generateLockID()
	deadline := time.Now().Add(lockWaitMax)
	for {
		acquired, err := tryLock(lockPath, myUUID)
		if err != nil {
			return nil, "", err
		}
		if acquired {
			release := func() {
				// Liberar solo si el lock sigue siendo nuestro (TOCTOU-safe).
				data, err := os.ReadFile(lockPath)
				if err != nil {
					return
				}
				fields := strings.Fields(string(data))
				if len(fields) == 3 {
					if p, err := strconv.Atoi(fields[0]); err == nil && p == pid && fields[2] == myUUID {
						os.Remove(lockPath)
					}
				}
			}
			return release, myUUID, nil
		}
		if time.Now().After(deadline) {
			return nil, "", fmt.Errorf("timeout esperando el lock de memoria (%s)", lockPath)
		}
		// Jitter anti-thundering-herd: sleep 100-150ms aleatorio para que
		// los procesos waiting no despierten todos al mismo tiempo.
		jitter := time.Duration(rand.Intn(50)) * time.Millisecond
		time.Sleep(100*time.Millisecond + jitter)
	}
}

// refreshLock actualiza el timestamp del lock file (heartbeat) para evitar
// que otros procesos lo reclamen como stale mientras trabajamos.
func refreshLock(lockPath, lockID string) {
	content := fmt.Sprintf("%d %s %s",
		os.Getpid(),
		time.Now().UTC().Format(time.RFC3339),
		lockID,
	)
	os.WriteFile(lockPath, []byte(content), 0600)
}

func tryLock(lockPath, lockID string) (bool, error) {
	data, err := os.ReadFile(lockPath)
	if os.IsNotExist(err) {
		return writeLock(lockPath, lockID)
	}
	// En Windows, leer un lock que otro proceso está eliminando devuelve
	// ERROR_SHARING_VIOLATION/ACCESS_DENIED: se trata como contención y se
	// reintenta, no como error fatal.
	if isSharingViolation(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fields := strings.Fields(string(data))
	// Aceptar tanto formato viejo (2 campos: PID TS) como nuevo (3 campos: PID TS UUID).
	// Locks viejos sin UUID se tratan como stale para limpieza automática.
	if len(fields) != 3 {
		os.Remove(lockPath)
		return writeLock(lockPath, lockID)
	}
	pid, err1 := strconv.Atoi(fields[0])
	ts, err2 := time.Parse(time.RFC3339, fields[1])
	if err1 != nil || err2 != nil {
		os.Remove(lockPath)
		return writeLock(lockPath, lockID)
	}
	// Stale: PID muerto, lock viejo, o timestamp en el futuro (clock skew).
	futureSkew := ts.After(time.Now().Add(lockMaxClockSkew))
	if !pidAlive(pid) || time.Since(ts) > lockStaleAfter || futureSkew {
		os.Remove(lockPath)
		return writeLock(lockPath, lockID)
	}
	return false, nil
}

func writeLock(lockPath, lockID string) (bool, error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) || isSharingViolation(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	content := fmt.Sprintf("%d %s %s", os.Getpid(), time.Now().UTC().Format(time.RFC3339), lockID)
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return false, err
	}
	return f.Close() == nil, nil
}
