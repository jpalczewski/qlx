package ui

import (
	"net/http"

	"github.com/erxyi/qlx/internal/shared/webutil"
	"github.com/erxyi/qlx/internal/store"
)

func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	data, _ := s.containerViewModel("")
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainer(w http.ResponseWriter, r *http.Request) {
	data, ok := s.containerViewModel(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerCreate(w http.ResponseWriter, r *http.Request) {
	parentID := r.FormValue("parent_id")
	name := r.FormValue("name")
	description := r.FormValue("description")

	container := s.store.CreateContainer(parentID, name, description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	data, _ := s.containerViewModel(container.ID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.FormValue("name")
	description := r.FormValue("description")

	_, err := s.store.UpdateContainer(id, name, description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(id)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	container := s.store.GetContainer(id)
	var parentID string
	if container != nil {
		parentID = container.ParentID
	}

	err := s.store.DeleteContainer(id)
	if err != nil {
		if err == store.ErrContainerNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(parentID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleItem(w http.ResponseWriter, r *http.Request) {
	data, ok := s.itemDetailViewModel(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "item", data)
}

func (s *Server) HandleItemCreate(w http.ResponseWriter, r *http.Request) {
	containerID := r.FormValue("container_id")
	name := r.FormValue("name")
	description := r.FormValue("description")

	s.store.CreateItem(containerID, name, description)
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, ok := s.containerViewModel(containerID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "containers", data)
}

func (s *Server) HandleItemUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.FormValue("name")
	description := r.FormValue("description")

	item, err := s.store.UpdateItem(id, name, description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.itemDetailViewModel(item.ID)
	s.render(w, r, "item", data)
}

func (s *Server) HandleItemDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	item := s.store.GetItem(id)
	var containerID string
	if item != nil {
		containerID = item.ContainerID
	}

	err := s.store.DeleteItem(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}

	data, _ := s.containerViewModel(containerID)
	s.render(w, r, "containers", data)
}

func (s *Server) HandleContainerMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newParentID := r.FormValue("parent_id")

	if err := s.store.MoveContainer(id, newParentID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) HandleItemMove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	newContainerID := r.FormValue("container_id")

	if err := s.store.MoveItem(id, newContainerID); err != nil {
		webutil.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !webutil.SaveOrFail(w, s.store.Save) {
		return
	}
	webutil.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
