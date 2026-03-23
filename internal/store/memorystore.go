package store

// MemoryStore is a minimal in-memory Store implementation for test compilation.
// TODO: Replace with SQLite-backed tests in Tasks 13/14.
type MemoryStore struct {
	printers  map[string]*PrinterConfig
	templates map[string]*Template
}

// NewMemoryStore returns a minimal in-memory store stub for tests.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		printers:  make(map[string]*PrinterConfig),
		templates: make(map[string]*Template),
	}
}

func (m *MemoryStore) GetContainer(id string) *Container { return nil }
func (m *MemoryStore) CreateContainer(parentID, name, desc, color, icon string) *Container {
	return &Container{ID: "stub", Name: name}
}
func (m *MemoryStore) UpdateContainer(id, name, desc, color, icon string) (*Container, error) {
	return &Container{ID: id, Name: name}, nil
}
func (m *MemoryStore) DeleteContainer(id string) (string, error)     { return "", nil }
func (m *MemoryStore) MoveContainer(id, newParentID string) error    { return nil }
func (m *MemoryStore) ContainerChildren(parentID string) []Container { return nil }
func (m *MemoryStore) ContainerItems(containerID string) []Item      { return nil }
func (m *MemoryStore) ContainerPath(id string) []Container           { return nil }
func (m *MemoryStore) AllContainers() []Container                    { return nil }
func (m *MemoryStore) GetItem(id string) *Item                       { return nil }
func (m *MemoryStore) CreateItem(containerID, name, desc string, qty int, color, icon string) *Item {
	return &Item{ID: "stub", Name: name, ContainerID: containerID}
}
func (m *MemoryStore) UpdateItem(id, name, desc string, qty int, color, icon string) (*Item, error) {
	return &Item{ID: id, Name: name}, nil
}
func (m *MemoryStore) DeleteItem(id string) (string, error)  { return "", nil }
func (m *MemoryStore) MoveItem(id, containerID string) error { return nil }
func (m *MemoryStore) GetTag(id string) *Tag                 { return nil }
func (m *MemoryStore) CreateTag(parentID, name, color, icon string) *Tag {
	return &Tag{ID: "stub", Name: name}
}
func (m *MemoryStore) UpdateTag(id, name, color, icon string) (*Tag, error) {
	return &Tag{ID: id, Name: name}, nil
}
func (m *MemoryStore) DeleteTag(id string) (string, error)                { return "", nil }
func (m *MemoryStore) MoveTag(id, newParentID string) error               { return nil }
func (m *MemoryStore) AllTags() []Tag                                     { return nil }
func (m *MemoryStore) TagChildren(parentID string) []Tag                  { return nil }
func (m *MemoryStore) TagPath(id string) []Tag                            { return nil }
func (m *MemoryStore) TagDescendants(id string) []string                  { return nil }
func (m *MemoryStore) AddItemTag(itemID, tagID string) error              { return nil }
func (m *MemoryStore) RemoveItemTag(itemID, tagID string) error           { return nil }
func (m *MemoryStore) AddContainerTag(containerID, tagID string) error    { return nil }
func (m *MemoryStore) RemoveContainerTag(containerID, tagID string) error { return nil }
func (m *MemoryStore) ItemsByTag(tagID string) []Item                     { return nil }
func (m *MemoryStore) ContainersByTag(tagID string) []Container           { return nil }
func (m *MemoryStore) ResolveTagIDs(ids []string) []Tag                   { return nil }
func (m *MemoryStore) TagItemStats(id string) (int, int, error)           { return 0, 0, nil }
func (m *MemoryStore) BulkMove(itemIDs, containerIDs []string, targetID string) []BulkError {
	return nil
}
func (m *MemoryStore) BulkDelete(itemIDs, containerIDs []string) ([]string, []BulkError) {
	return nil, nil
}
func (m *MemoryStore) BulkAddTag(itemIDs, containerIDs []string, tagID string) error { return nil }
func (m *MemoryStore) SearchContainers(query string) []Container                     { return nil }
func (m *MemoryStore) SearchItems(query string) []Item                               { return nil }
func (m *MemoryStore) SearchTags(query string) []Tag                                 { return nil }

func (m *MemoryStore) AllPrinters() []PrinterConfig {
	var result []PrinterConfig
	for _, p := range m.printers {
		result = append(result, *p)
	}
	return result
}

func (m *MemoryStore) GetPrinter(id string) *PrinterConfig {
	p, ok := m.printers[id]
	if !ok {
		return nil
	}
	return p
}

func (m *MemoryStore) AddPrinter(name, enc, model, transport, address string) *PrinterConfig {
	p := &PrinterConfig{
		ID:        "p-" + name,
		Name:      name,
		Encoder:   enc,
		Model:     model,
		Transport: transport,
		Address:   address,
	}
	m.printers[p.ID] = p
	return p
}

func (m *MemoryStore) DeletePrinter(id string) error {
	delete(m.printers, id)
	return nil
}

func (m *MemoryStore) UpdatePrinterOffset(id string, offsetX, offsetY int) error {
	p, ok := m.printers[id]
	if !ok {
		return ErrPrinterNotFound
	}
	p.OffsetX = offsetX
	p.OffsetY = offsetY
	return nil
}

func (m *MemoryStore) AllTemplates() []Template {
	var result []Template
	for _, t := range m.templates {
		result = append(result, *t)
	}
	return result
}

func (m *MemoryStore) GetTemplate(id string) *Template {
	t, ok := m.templates[id]
	if !ok {
		return nil
	}
	return t
}

func (m *MemoryStore) CreateTemplate(name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error) {
	t := &Template{
		ID:       "tmpl-" + name,
		Name:     name,
		Tags:     tags,
		Target:   target,
		WidthMM:  widthMM,
		HeightMM: heightMM,
		WidthPx:  widthPx,
		HeightPx: heightPx,
		Elements: elements,
	}
	m.templates[t.ID] = t
	return t, nil
}

func (m *MemoryStore) UpdateTemplate(id, name string, tags []string, target string, widthMM, heightMM float64, widthPx, heightPx int, elements string) (*Template, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	t.Name = name
	t.Tags = tags
	t.Target = target
	t.WidthMM = widthMM
	t.HeightMM = heightMM
	t.WidthPx = widthPx
	t.HeightPx = heightPx
	t.Elements = elements
	return t, nil
}

func (m *MemoryStore) DeleteTemplate(id string) error {
	delete(m.templates, id)
	return nil
}

func (m *MemoryStore) ExportData() (map[string]*Container, map[string]*Item) { return nil, nil }
func (m *MemoryStore) AllItems() []Item                                      { return nil }
func (m *MemoryStore) Close() error                                          { return nil }
