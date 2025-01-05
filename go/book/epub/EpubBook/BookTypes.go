package EpubBook

type Section struct {
	Title     string `json:"title"`      // 标题
	Content   string `json:"content"`    // 类容
	PlayOrder int    `json:"play_order"` // 顺序
}
type CatLog struct {
	Id        string `json:"id"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	PlayOrder int    `json:"play_order"`
}
