package domain

type Item any

type ItemWithFile struct {
	Item            Item
	UseFileFieldKey string
	File            FileItem
}
