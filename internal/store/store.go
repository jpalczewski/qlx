package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrContainerNotFound    = errors.New("container not found")
	ErrItemNotFound         = errors.New("item not found")
	ErrCycleDetected        = errors.New("cycle detected")
	ErrInvalidParent        = errors.New("invalid parent container")
	ErrInvalidContainer     = errors.New("invalid container for item")
	ErrContainerHasChildren = errors.New("container has children")
	ErrContainerHasItems    = errors.New("container has items")
	ErrPrinterNotFound      = errors.New("printer not found")
)

// storeData is the legacy monolithic serialization format (V1).
type storeData struct {
	Version    int                       `json:"version"`
	Containers map[string]*Container     `json:"containers"`
	Items      map[string]*Item          `json:"items"`
	Printers   map[string]*PrinterConfig `json:"printers"`
	Templates  map[string]*Template      `json:"templates"`
	Assets     map[string]*Asset         `json:"assets"`
	Tags       map[string]*Tag           `json:"tags"`
}

// collectionFlag is a bitmask identifying which collections have been modified.
type collectionFlag uint8

const (
	dirtyContainers collectionFlag = 1 << iota
	dirtyItems
	dirtyPrinters
	dirtyTemplates
	dirtyAssets
	dirtyTags
)

// Store is the in-memory data store with disk persistence.
type Store struct {
	mu         sync.RWMutex
	dataDir    string // directory for partitioned files (empty for MemoryStore)
	assetsDir  string
	dirty      collectionFlag
	containers map[string]*Container
	items      map[string]*Item
	printers   map[string]*PrinterConfig
	templates  map[string]*Template
	assets     map[string]*Asset
	tags       map[string]*Tag
}

// collectionFiles maps dirty flags to their partition filenames.
var collectionFiles = []struct {
	flag collectionFlag
	name string
}{
	{dirtyContainers, "containers.json"},
	{dirtyItems, "items.json"},
	{dirtyPrinters, "printers.json"},
	{dirtyTemplates, "templates.json"},
	{dirtyAssets, "assets.json"},
	{dirtyTags, "tags.json"},
}

// loadLegacyData reads a monolithic data.json, runs pending migrations, and returns a storeData.
// Returns nil, nil when the file does not exist or is empty.
func loadLegacyData(path string) (*storeData, error) {
	raw, fileVersion, err := loadRaw(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(raw) == 0 {
		return nil, nil
	}

	fileData, err := migrateIfNeeded(path, raw, fileVersion)
	if err != nil {
		return nil, err
	}

	var d storeData
	if err := json.Unmarshal(fileData, &d); err != nil {
		return nil, err
	}

	return &d, nil
}

// migrateIfNeeded runs pending migrations when fileVersion < currentVersion,
// persists the result atomically, and returns the (possibly migrated) JSON bytes.
func migrateIfNeeded(path string, raw map[string]any, fileVersion int) ([]byte, error) {
	if fileVersion < currentVersion {
		migrated, _, err := runMigrations(path, raw, fileVersion)
		if err != nil {
			return nil, err
		}
		if err := writeAtomic(path, migrated); err != nil {
			return nil, err
		}
		return migrated, nil
	}
	return json.Marshal(raw)
}

// loadPartitionedData loads collections from individual JSON files in dataDir.
func loadPartitionedData(dataDir string) (*storeData, error) {
	d := &storeData{}

	type target struct {
		name string
		dest any
	}
	targets := []target{
		{"containers.json", &d.Containers},
		{"items.json", &d.Items},
		{"printers.json", &d.Printers},
		{"templates.json", &d.Templates},
		{"assets.json", &d.Assets},
		{"tags.json", &d.Tags},
	}

	for _, t := range targets {
		path := filepath.Join(dataDir, t.name)
		//nolint:gosec // G304: path from trusted CLI input
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		if err := json.Unmarshal(raw, t.dest); err != nil {
			return nil, fmt.Errorf("loading %s: %w", t.name, err)
		}
	}

	return d, nil
}

// NewStore creates a new disk-backed store. The path argument is the legacy data.json path;
// the directory containing it becomes the data directory for partitioned files.
func NewStore(path string) (*Store, error) {
	dataDir := filepath.Dir(path)
	assetsDir := filepath.Join(dataDir, "assets")
	s := &Store{
		dataDir:    dataDir,
		assetsDir:  assetsDir,
		containers: make(map[string]*Container),
		items:      make(map[string]*Item),
		printers:   make(map[string]*PrinterConfig),
		templates:  make(map[string]*Template),
		assets:     make(map[string]*Asset),
		tags:       make(map[string]*Tag),
	}

	metaPath := filepath.Join(dataDir, "meta.json")
	if _, err := os.Stat(metaPath); err == nil {
		// Partitioned format: load individual collection files.
		d, err := loadPartitionedData(dataDir)
		if err != nil {
			return nil, err
		}
		s.applyStoreData(d)
		return s, nil
	}

	// Check if legacy monolithic file exists.
	if _, err := os.Stat(path); err == nil {
		// Legacy monolithic format: load, migrate, then write partitioned.
		d, err := loadLegacyData(path)
		if err != nil {
			return nil, err
		}

		if d != nil {
			s.applyStoreData(d)

			// Migrate to partitioned format.
			if err := s.writeAllPartitions(); err != nil {
				return nil, fmt.Errorf("migrating to partitioned format: %w", err)
			}
			if err := writeAtomic(metaPath, []byte(`{"format":"partitioned","schema_version":1}`)); err != nil {
				return nil, err
			}
			// Back up the legacy file.
			backupPath := path + ".migrated"
			_ = os.Rename(path, backupPath)
		}
	}

	// Ensure meta.json exists for new stores so subsequent loads use partitioned format.
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		//nolint:gosec // G301: intentional permissions for data directory
		_ = os.MkdirAll(dataDir, 0755)
		_ = writeAtomic(metaPath, []byte(`{"format":"partitioned","schema_version":1}`))
	}

	return s, nil
}

// applyStoreData populates the store from loaded data.
func (s *Store) applyStoreData(d *storeData) {
	if d.Containers != nil {
		s.containers = d.Containers
	}
	if d.Items != nil {
		s.items = d.Items
	}
	if d.Printers != nil {
		s.printers = d.Printers
	}
	if d.Templates != nil {
		s.templates = d.Templates
	}
	if d.Assets != nil {
		s.assets = d.Assets
	}
	if d.Tags != nil {
		s.tags = d.Tags
	}
}

// Save writes only the modified collections to their individual JSON files.
func (s *Store) Save() error {
	if s.dataDir == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dirty == 0 {
		return nil
	}

	//nolint:gosec // G301: intentional permissions for data directory
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	collectionData := map[collectionFlag]any{
		dirtyContainers: s.containers,
		dirtyItems:      s.items,
		dirtyPrinters:   s.printers,
		dirtyTemplates:  s.templates,
		dirtyAssets:     s.assets,
		dirtyTags:       s.tags,
	}

	for _, cf := range collectionFiles {
		if s.dirty&cf.flag == 0 {
			continue
		}
		data, err := json.Marshal(collectionData[cf.flag])
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(s.dataDir, cf.name), data); err != nil {
			return err
		}
	}

	s.dirty = 0
	return nil
}

// writeAllPartitions writes all collections to partitioned files (used during migration).
func (s *Store) writeAllPartitions() error {
	//nolint:gosec // G301: intentional permissions for data directory
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}

	collectionData := map[collectionFlag]any{
		dirtyContainers: s.containers,
		dirtyItems:      s.items,
		dirtyPrinters:   s.printers,
		dirtyTemplates:  s.templates,
		dirtyAssets:     s.assets,
		dirtyTags:       s.tags,
	}

	for _, cf := range collectionFiles {
		data, err := json.Marshal(collectionData[cf.flag])
		if err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(s.dataDir, cf.name), data); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateContainer(parentID, name, description string) *Container {
	s.mu.Lock()
	defer s.mu.Unlock()

	c := &Container{
		ID:          uuid.New().String(),
		ParentID:    parentID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		TagIDs:      []string{},
	}
	s.containers[c.ID] = c
	s.dirty |= dirtyContainers
	return c
}

func (s *Store) GetContainer(id string) *Container {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containers[id]
}

func (s *Store) UpdateContainer(id, name, description string) (*Container, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.containers[id]
	if !ok {
		return nil, ErrContainerNotFound
	}

	c.Name = name
	c.Description = description
	s.dirty |= dirtyContainers
	return c, nil
}

func (s *Store) DeleteContainer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.containers[id]; !ok {
		return ErrContainerNotFound
	}

	for _, c := range s.containers {
		if c.ParentID == id {
			return ErrContainerHasChildren
		}
	}

	for _, item := range s.items {
		if item.ContainerID == id {
			return ErrContainerHasItems
		}
	}

	delete(s.containers, id)
	s.dirty |= dirtyContainers
	return nil
}

// CreateItem creates a new item in the given container. If quantity is less than 1 it defaults to 1.
func (s *Store) CreateItem(containerID, name, description string, quantity int) *Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	if quantity < 1 {
		quantity = 1
	}

	item := &Item{
		ID:          uuid.New().String(),
		ContainerID: containerID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		Quantity:    quantity,
		TagIDs:      []string{},
	}
	s.items[item.ID] = item
	s.dirty |= dirtyItems
	return item
}

func (s *Store) GetItem(id string) *Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[id]
}

// UpdateItem updates an item's name, description, and quantity. If quantity is less than 1
// the existing quantity is preserved.
func (s *Store) UpdateItem(id, name, description string, quantity int) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return nil, ErrItemNotFound
	}

	item.Name = name
	item.Description = description
	if quantity >= 1 {
		item.Quantity = quantity
	}
	s.dirty |= dirtyItems
	return item, nil
}

func (s *Store) DeleteItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrItemNotFound
	}

	delete(s.items, id)
	s.dirty |= dirtyItems
	return nil
}

func (s *Store) ContainerPath(id string) []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var path []Container
	current := s.containers[id]
	for current != nil {
		path = append([]Container{*current}, path...)
		if current.ParentID == "" {
			break
		}
		current = s.containers[current.ParentID]
	}
	return path
}

func (s *Store) ContainerChildren(id string) []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var children []Container
	for _, c := range s.containers {
		if c.ParentID == id {
			children = append(children, *c)
		}
	}
	return children
}

func (s *Store) ContainerItems(id string) []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []Item
	for _, item := range s.items {
		if item.ContainerID == id {
			items = append(items, *item)
		}
	}
	return items
}

func (s *Store) MoveItem(itemID, newContainerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[itemID]
	if !ok {
		return ErrItemNotFound
	}

	if newContainerID != "" {
		if _, ok := s.containers[newContainerID]; !ok {
			return ErrInvalidContainer
		}
	}

	item.ContainerID = newContainerID
	s.dirty |= dirtyItems
	return nil
}

func (s *Store) MoveContainer(containerID, newParentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	container, ok := s.containers[containerID]
	if !ok {
		return ErrContainerNotFound
	}

	if newParentID != "" {
		if _, ok := s.containers[newParentID]; !ok {
			return ErrInvalidParent
		}
	}

	if newParentID == containerID {
		return ErrCycleDetected
	}

	parent := s.containers[newParentID]
	for parent != nil {
		if parent.ID == containerID {
			return ErrCycleDetected
		}
		parent = s.containers[parent.ParentID]
	}

	container.ParentID = newParentID
	s.dirty |= dirtyContainers
	return nil
}

func NewMemoryStore() *Store {
	return &Store{
		containers: make(map[string]*Container),
		items:      make(map[string]*Item),
		printers:   make(map[string]*PrinterConfig),
		templates:  make(map[string]*Template),
		assets:     make(map[string]*Asset),
		tags:       make(map[string]*Tag),
	}
}

func (s *Store) AddPrinter(name, encoder, model, transport, address string) *PrinterConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := &PrinterConfig{
		ID:        uuid.New().String(),
		Name:      name,
		Encoder:   encoder,
		Model:     model,
		Transport: transport,
		Address:   address,
	}
	s.printers[p.ID] = p
	s.dirty |= dirtyPrinters
	return p
}

func (s *Store) GetPrinter(id string) *PrinterConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.printers[id]
}

func (s *Store) DeletePrinter(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.printers[id]; !ok {
		return ErrPrinterNotFound
	}

	delete(s.printers, id)
	s.dirty |= dirtyPrinters
	return nil
}

func (s *Store) AllPrinters() []PrinterConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var printers []PrinterConfig
	for _, p := range s.printers {
		printers = append(printers, *p)
	}
	return printers
}

func (s *Store) AllItems() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var items []Item
	for _, item := range s.items {
		items = append(items, *item)
	}
	return items
}

func (s *Store) AllContainers() []Container {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var containers []Container
	for _, c := range s.containers {
		containers = append(containers, *c)
	}
	return containers
}

func (s *Store) ExportData() (map[string]*Container, map[string]*Item) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	containersCopy := make(map[string]*Container)
	for k, v := range s.containers {
		c := *v
		containersCopy[k] = &c
	}

	itemsCopy := make(map[string]*Item)
	for k, v := range s.items {
		i := *v
		itemsCopy[k] = &i
	}

	return containersCopy, itemsCopy
}

func (s *Store) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) *Template {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	t := &Template{
		ID:        uuid.New().String(),
		Name:      name,
		Tags:      tags,
		Target:    target,
		WidthMM:   widthMM,
		HeightMM:  heightMM,
		WidthPx:   widthPx,
		HeightPx:  heightPx,
		Elements:  elements,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.templates[t.ID] = t
	s.dirty |= dirtyTemplates
	return t
}

func (s *Store) GetTemplate(id string) *Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates[id]
}

func (s *Store) AllTemplates() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var templates []Template
	for _, t := range s.templates {
		templates = append(templates, *t)
	}
	return templates
}

func (s *Store) SaveTemplate(t Template) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.UpdatedAt = time.Now()
	s.templates[t.ID] = &t
	s.dirty |= dirtyTemplates
}

func (s *Store) DeleteTemplate(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.templates, id)
	s.dirty |= dirtyTemplates
}

func (s *Store) SaveAsset(name, mimeType string, data []byte) (*Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a := &Asset{
		ID:        uuid.New().String(),
		Name:      name,
		MimeType:  mimeType,
		CreatedAt: time.Now(),
	}

	if s.assetsDir != "" {
		//nolint:gosec // G301: intentional permissions for assets directory
		if err := os.MkdirAll(s.assetsDir, 0755); err != nil {
			return nil, err
		}
		//nolint:gosec // G306: intentional permissions for asset file
		if err := os.WriteFile(filepath.Join(s.assetsDir, a.ID+".bin"), data, 0644); err != nil {
			return nil, err
		}
	}

	s.assets[a.ID] = a
	s.dirty |= dirtyAssets
	return a, nil
}

func (s *Store) GetAsset(id string) *Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.assets[id]
}

func (s *Store) AssetData(id string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.assets[id]; !ok {
		return nil, errors.New("asset not found")
	}

	//nolint:gosec // G304: asset ID is a UUID generated by us
	return os.ReadFile(filepath.Join(s.assetsDir, id+".bin"))
}

func (s *Store) DeleteAsset(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.assets[id]; !ok {
		return
	}

	_ = os.Remove(filepath.Join(s.assetsDir, id+".bin"))
	delete(s.assets, id)
	s.dirty |= dirtyAssets
}

func (s *Store) AllAssets() []Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var assets []Asset
	for _, a := range s.assets {
		assets = append(assets, *a)
	}
	return assets
}
