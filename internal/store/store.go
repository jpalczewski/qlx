package store

import (
	"encoding/json"
	"errors"
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

type storeData struct {
	Containers map[string]*Container     `json:"containers"`
	Items      map[string]*Item          `json:"items"`
	Printers   map[string]*PrinterConfig `json:"printers"`
	Templates  map[string]*Template      `json:"templates"`
	Assets     map[string]*Asset         `json:"assets"`
}

type Store struct {
	mu         sync.RWMutex
	path       string
	assetsDir  string
	containers map[string]*Container
	items      map[string]*Item
	printers   map[string]*PrinterConfig
	templates  map[string]*Template
	assets     map[string]*Asset
}

func NewStore(path, assetsDir string) (*Store, error) {
	s := &Store{
		path:       path,
		assetsDir:  assetsDir,
		containers: make(map[string]*Container),
		items:      make(map[string]*Item),
		printers:   make(map[string]*PrinterConfig),
		templates:  make(map[string]*Template),
		assets:     make(map[string]*Asset),
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if len(fileData) == 0 {
		return s, nil
	}

	var d storeData
	if err := json.Unmarshal(fileData, &d); err != nil {
		return nil, err
	}

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

	return s, nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(&storeData{
		Containers: s.containers,
		Items:      s.items,
		Printers:   s.printers,
		Templates:  s.templates,
		Assets:     s.assets,
	})
	if err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, s.path)
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
	}
	s.containers[c.ID] = c
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
	return nil
}

func (s *Store) CreateItem(containerID, name, description string) *Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := &Item{
		ID:          uuid.New().String(),
		ContainerID: containerID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}
	s.items[item.ID] = item
	return item
}

func (s *Store) GetItem(id string) *Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[id]
}

func (s *Store) UpdateItem(id, name, description string) (*Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return nil, ErrItemNotFound
	}

	item.Name = name
	item.Description = description
	return item, nil
}

func (s *Store) DeleteItem(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrItemNotFound
	}

	delete(s.items, id)
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
	return nil
}

func NewMemoryStore() *Store {
	return &Store{
		containers: make(map[string]*Container),
		items:      make(map[string]*Item),
		printers:   make(map[string]*PrinterConfig),
		templates:  make(map[string]*Template),
		assets:     make(map[string]*Asset),
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
