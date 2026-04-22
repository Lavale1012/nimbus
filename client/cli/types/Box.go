package types

type BoxEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type ListBoxesResponse struct {
	Boxes []BoxEntry `json:"boxes"`
}
