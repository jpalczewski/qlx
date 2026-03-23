package store

import "errors"

var (
	ErrContainerNotFound    = errors.New("container not found")
	ErrItemNotFound         = errors.New("item not found")
	ErrTagNotFound          = errors.New("tag not found")
	ErrPrinterNotFound      = errors.New("printer not found")
	ErrTemplateNotFound     = errors.New("template not found")
	ErrContainerHasChildren = errors.New("container has children")
	ErrContainerHasItems    = errors.New("container has items")
	ErrTagHasChildren       = errors.New("tag has children")
	ErrCycleDetected        = errors.New("cycle detected")
	ErrInvalidParent        = errors.New("invalid parent container")
	ErrInvalidContainer     = errors.New("invalid container for item")
)
