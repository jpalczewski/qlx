package ui

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

// HandleTags renders the tag tree page. Accepts optional ?parent= query parameter.
func (s *Server) HandleTags(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("parent")

	var parent *store.Tag
	var path []store.Tag
	if parentID != "" {
		parent = s.store.GetTag(parentID)
		if parent == nil {
			http.NotFound(w, r)
			return
		}
		// Path includes the parent itself; exclude the last element for breadcrumb display
		fullPath := s.store.TagPath(parentID)
		if len(fullPath) > 0 {
			path = fullPath[:len(fullPath)-1]
		}
	}

	tags := s.store.TagChildren(parentID)

	data := TagTreeData{
		Tags:   tags,
		Parent: parent,
		Path:   path,
	}
	s.render(w, r, "tags", data)
}

// HandleTagCreate handles POST /ui/actions/tags. Creates a new tag and responds with
// the tag-list-item partial for HTMX requests, or redirects for plain requests.
func (s *Server) HandleTagCreate(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")          //nolint:gosec // G120: internal tool, no untrusted input
	parentID := r.FormValue("parent_id") //nolint:gosec // G120: internal tool, no untrusted input

	tag := s.store.CreateTag(parentID, name)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	if webutil.IsHTMX(r) {
		s.renderPartial(w, r, "tags", "tag-list-item", tag)
		return
	}
	if parentID != "" {
		http.Redirect(w, r, "/ui/tags?parent="+parentID, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/ui/tags", http.StatusSeeOther)
	}
}

// HandleTagUpdate handles PUT /ui/actions/tags/{id}. Updates a tag's name.
func (s *Server) HandleTagUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.FormValue("name") //nolint:gosec // G120: internal tool, no untrusted input

	tag, err := s.store.UpdateTag(id, name)
	if err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	parentID := tag.ParentID
	if parentID != "" {
		http.Redirect(w, r, "/ui/tags?parent="+parentID, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/ui/tags", http.StatusSeeOther)
	}
}

// HandleTagDelete handles DELETE /ui/actions/tags/{id}. Deletes a tag.
func (s *Server) HandleTagDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tag := s.store.GetTag(id)
	var parentID string
	if tag != nil {
		parentID = tag.ParentID
	}

	if err := s.store.DeleteTag(id); err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	if parentID != "" {
		http.Redirect(w, r, "/ui/tags?parent="+parentID, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/ui/tags", http.StatusSeeOther)
	}
}

// HandleTagMove handles POST /ui/actions/tags/{id}/move. Moves a tag to a new parent.
func (s *Server) HandleTagMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newParentID := r.FormValue("parent_id") //nolint:gosec // G120: internal tool, no untrusted input

	if err := s.store.MoveTag(id, newParentID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// resolveTagIDs looks up each tag ID and returns the corresponding Tag objects.
func (s *Server) resolveTagIDs(ids []string) []store.Tag {
	tags := make([]store.Tag, 0, len(ids))
	for _, id := range ids {
		if t := s.store.GetTag(id); t != nil {
			tags = append(tags, *t)
		}
	}
	return tags
}

// HandleItemTagAdd handles POST /ui/actions/items/{id}/tags.
// Adds a tag to an item and returns the updated tag-chips partial.
func (s *Server) HandleItemTagAdd(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.FormValue("tag_id") //nolint:gosec // G120: internal tool, no untrusted input

	if err := s.store.AddItemTag(id, tagID); err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	item := s.store.GetItem(id)
	if item == nil {
		http.NotFound(w, r)
		return
	}

	data := TagChipsData{
		ObjectID:   id,
		ObjectType: "item",
		Tags:       s.resolveTagIDs(item.TagIDs),
	}
	s.renderPartial(w, r, "tags", "tag-chips", data)
}

// HandleItemTagRemove handles DELETE /ui/actions/items/{id}/tags/{tag_id}.
// Removes a tag from an item and returns the updated tag-chips partial.
func (s *Server) HandleItemTagRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.PathValue("tag_id")

	if err := s.store.RemoveItemTag(id, tagID); err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	item := s.store.GetItem(id)
	if item == nil {
		http.NotFound(w, r)
		return
	}

	data := TagChipsData{
		ObjectID:   id,
		ObjectType: "item",
		Tags:       s.resolveTagIDs(item.TagIDs),
	}
	s.renderPartial(w, r, "tags", "tag-chips", data)
}

// HandleContainerTagAdd handles POST /ui/actions/containers/{id}/tags.
// Adds a tag to a container and returns the updated tag-chips partial.
func (s *Server) HandleContainerTagAdd(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.FormValue("tag_id") //nolint:gosec // G120: internal tool, no untrusted input

	if err := s.store.AddContainerTag(id, tagID); err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	container := s.store.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	data := TagChipsData{
		ObjectID:   id,
		ObjectType: "container",
		Tags:       s.resolveTagIDs(container.TagIDs),
	}
	s.renderPartial(w, r, "tags", "tag-chips", data)
}

// HandleContainerTagRemove handles DELETE /ui/actions/containers/{id}/tags/{tag_id}.
// Removes a tag from a container and returns the updated tag-chips partial.
func (s *Server) HandleContainerTagRemove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tagID := r.PathValue("tag_id")

	if err := s.store.RemoveContainerTag(id, tagID); err != nil {
		webutil.WriteStoreErrorText(w, err)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	container := s.store.GetContainer(id)
	if container == nil {
		http.NotFound(w, r)
		return
	}

	data := TagChipsData{
		ObjectID:   id,
		ObjectType: "container",
		Tags:       s.resolveTagIDs(container.TagIDs),
	}
	s.renderPartial(w, r, "tags", "tag-chips", data)
}
