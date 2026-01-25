package mobi

// PALMDOC_HEADER PalmDOC 头部信息
// 包含压缩类型、文本记录数等关键信息
type PALMDOC_HEADER struct {
	Compression    int `json:"compression"`    // 压缩类型：1=无压缩, 2=PalmDOC, 17480=HUFF/CDIC
	NumTextRecords int `json:"numTextRecords"` // 文本记录的数量
	RecordSize     int `json:"recordSize"`     // 记录大小（通常为 4096）
	Encryption     int `json:"encryption"`     // 加密类型
}

// MOBI_HEADER Mobi 格式核心头部
// 包含版本号、标题偏移、字符编码、Huffman 表偏移等核心元数据
type MOBI_HEADER struct {
	Magic          string `json:"magic"`          // 标识符 "MOBI"
	Length         uint32 `json:"length"`         // 头部长度
	Type           uint32 `json:"type"`           // Mobi 类型
	Encoding       uint32 `json:"encoding"`       // 文本编码：1252=CP1252, 65001=UTF-8
	Uid            uint32 `json:"uid"`            // 唯一 ID
	Version        uint32 `json:"version"`        // 格式版本号
	TitleOffset    uint32 `json:"titleOffset"`    // 标题在记录中的偏移量
	TitleLength    uint32 `json:"titleLength"`    // 标题长度
	LocaleRegion   uint32 `json:"localeRegion"`   // 地区代码
	LocaleLanguage uint32 `json:"localeLanguage"` // 语言代码
	ResourceStart  uint32 `json:"resourceStart"`  // 资源记录起始索引
	Huffcdic       uint32 `json:"huffcdic"`       // HUFF/CDIC 记录的索引
	NumHuffcdic    uint32 `json:"numHuffcdic"`    // HUFF/CDIC 记录数量
	ExthFlag       uint32 `json:"exthFlag"`       // EXTH 头部存在标志 (bit 6)
	TrailingFlags  uint32 `json:"trailingFlags"`  // 尾部数据标志
	Indx           uint32 `json:"indx"`           // 索引记录位置
	Title          []byte `json:"title"`          // 解析出的标题内容
	Language       string `json:"language"`       // 解析出的语言字符串
}

// EXTH_HEADER 扩展头部 (Extra Header)
// 包含书籍的详细元数据，如作者、ISBN、出版商等
type EXTH_HEADER struct {
	Magic   string    `json:"magic"`   // 标识符 "EXTH"
	Length  int       `json:"length"`  // 头部长度
	Count   int       `json:"count"`   // 记录数量
	ExthRow *EXTH_ROW `json:"exthRow"` // 解析后的扩展记录内容
}

// EXTH_ROW 具体的 EXTH 记录内容
type EXTH_ROW struct {
	Creator                  []string `json:"creator"`                  // 作者 (Type 100)
	Publisher                string   `json:"publisher"`                // 出版社 (Type 101)
	Description              string   `json:"description"`              // 简介 (Type 103)
	ISBN                     string   `json:"isbn"`                     // ISBN (Type 104)
	Subject                  []string `json:"subject"`                  // 主题/分类 (Type 105)
	Date                     string   `json:"date"`                     // 出版日期 (Type 106)
	Contributor              []string `json:"contributor"`              // 贡献者 (Type 108)
	Rights                   string   `json:"rights"`                   // 版权信息 (Type 109)
	SubjectCode              []string `json:"subjectCode"`              // 主题代码 (Type 110)
	Source                   []string `json:"source"`                   // 来源 (Type 112)
	ASIN                     string   `json:"asin"`                     // ASIN 号 (Type 113)
	Boundary                 uint32   `json:"boundary"`                 // KF8 边界偏移 (Type 121)
	FixedLayout              string   `json:"fixedLayout"`              // 固定布局 (Type 122)
	NumResources             uint32   `json:"numResources"`             // 资源数量 (Type 125)
	OriginalResolution       string   `json:"originalResolution"`       // 原始分辨率 (Type 126)
	ZeroGutter               string   `json:"zeroGutter"`               // 无装订线 (Type 127)
	ZeroMargin               string   `json:"zeroMargin"`               // 无边距 (Type 128)
	CoverURI                 string   `json:"coverURI"`                 // 封面 URI (Type 129)
	RegionMagnification      string   `json:"regionMagnification"`      // 区域放大 (Type 132)
	CoverOffset              uint32   `json:"coverOffset"`              // 封面图片偏移 (Type 201)
	ThumbnailOffset          uint32   `json:"thumbnailOffset"`          // 缩略图偏移 (Type 202)
	Title                    string   `json:"title"`                    // 标题 (Type 503)
	Language                 []string `json:"language"`                 // 语言 (Type 524)
	PageProgressionDirection string   `json:"pageProgressionDirection"` // 翻页方向 (Type 527)
}

// KF8_HEADER Kindle Format 8 (AZW3) 特有头部
type KF8_HEADER struct {
	ResourceStart int `json:"resourceStart"` // 资源起始位置
	Fdst          int `json:"fdst"`          // FDST 表位置
	NumFdst       int `json:"numFdst"`       // FDST 条目数
	Frag          int `json:"frag"`          // FRAG 表位置
	Skel          int `json:"skel"`          // SKEL 表位置
	Guide         int `json:"guide"`         // GUIDE 表位置
}

// HEADERS 汇总所有解析出的头部信息
type HEADERS struct {
	PalmdocHeader *PALMDOC_HEADER `json:"palmdoc"`
	PdbHeader     *PDB_HEADER     `json:"pdb"`
	MobiHeader    *MOBI_HEADER    `json:"mobi"`
	ExthHeader    *EXTH_HEADER    `json:"exth"`
	Kf8Header     *KF8_HEADER     `json:"kf8"`
}

// DecompressFunc 解压缩函数类型定义
type DecompressFunc func([]byte) ([]byte, error)

// RemoveTrailEntriesFunc 移除尾部数据函数类型定义
type RemoveTrailEntriesFunc func([]byte) []byte

// MOBI_BOOK 表示一本解析后的 Mobi 电子书
type MOBI_BOOK struct {
	Headers            *HEADERS               `json:"headers"` // 所有头部信息
	IS_KF8             bool                   `json:"isKf8"`   // 是否为 KF8/AZW3 格式
	Decompress         DecompressFunc         `json:"-"`       // 解压缩方法
	RemoveTrailEntries RemoveTrailEntriesFunc `json:"-"`       // 移除尾部数据方法
}

// PDB_HEADER PDB (Palm Database) 头部，位于文件最开始
type PDB_HEADER struct {
	Name       string `json:"name"`       // 数据库名称
	Type       string `json:"type"`       // 类型
	Creator    string `json:"creator"`    // 创建者 ID
	NumRecords int    `json:"numRecords"` // 记录总数
}

// HUFF_HEADER Huffman 压缩表头部
// 单独存在，不纳入 HEADERS
type HUFF_HEADER struct {
	Magic   string `json:"magic"`   // 标识符 "HUFF"
	Offset1 uint32 `json:"offset1"` // 表1 偏移量
	Offset2 uint32 `json:"offset2"` // 表2 偏移量
}

// CDIC_HEADER 压缩字典头部
// 单独存在，不纳入 HEADERS
type CDIC_HEADER struct {
	Magic      string `json:"magic"`      // 标识符 "CDIC"
	Length     uint32 `json:"length"`     // 头部长度
	NumEntries uint32 `json:"numEntries"` // 字典条目数
	CodeLength uint32 `json:"codeLength"` // 编码长度
}

// INDX_HEADER 索引记录头部
// 用于解析 Mobi 的索引结构 (如 NCX, SKEL 等)
type INDX_HEADER struct {
	Magic      string `json:"magic"`      // 标识符 "INDX"
	Length     uint32 `json:"length"`     // 头部长度
	Type       uint32 `json:"type"`       // 索引类型 (0=普通, 2=Inflection)
	Idxt       uint32 `json:"idxt"`       // IDXT 块的偏移量
	NumRecords uint32 `json:"numRecords"` // 索引记录数
	Encoding   uint32 `json:"encoding"`   // 编码
	Language   uint32 `json:"language"`   // 语言
	Total      uint32 `json:"total"`      // 总条目数
	Ordt       uint32 `json:"ordt"`       // ORDT 表偏移
	Ligt       uint32 `json:"ligt"`       // LIGT 表偏移
	NumLigt    uint32 `json:"numLigt"`    // LIGT 条目数
	NumCncx    uint32 `json:"numCncx"`    // CNCX 条目数
}

// TAGX_HEADER TAGX (Tag Index) 头部
// 定义了索引中 Tags 的控制字节和结构
type TAGX_HEADER struct {
	Magic           string `json:"magic"`           // 标识符 "TAGX"
	Length          uint32 `json:"length"`          // 头部长度
	NumControlBytes uint32 `json:"numControlBytes"` // 控制字节数
}

var (
	// MIME 类型映射表
	MIME = map[string]string{
		"XML":   "application/xml",
		"XHTML": "application/xhtml+xml",
		"HTML":  "text/html",
		"CSS":   "text/css",
		"SVG":   "image/svg+xml",
	}

	// PDB_HEADER_OFFSET_MAP PDB 头部字段偏移量映射
	// 格式: 字段名: {偏移量, 长度, 类型}
	PDB_HEADER_OFFSET_MAP = map[string][]interface{}{
		"name":       {0, 32, "string"},
		"type":       {60, 4, "string"},
		"creator":    {64, 4, "string"},
		"numRecords": {76, 2, "uint16"},
	}

	// PALMDOC_HEADER_OFFSET_MAP PalmDOC 头部字段偏移量映射
	PALMDOC_HEADER_OFFSET_MAP = map[string][]interface{}{
		"compression":    {0, 2, "uint16"},
		"numTextRecords": {8, 2, "uint16"},
		"recordSize":     {10, 2, "uint16"},
		"encryption":     {12, 2, "uint16"},
	}

	// MOBI_HEADER_OFFSET_MAP Mobi 头部字段偏移量映射
	MOBI_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":          {16, 4, "string"},
		"length":         {20, 4, "uint32"},
		"type":           {24, 4, "uint32"},
		"encoding":       {28, 4, "uint32"},
		"uid":            {32, 4, "uint32"},
		"version":        {36, 4, "uint32"},
		"titleOffset":    {84, 4, "uint32"},
		"titleLength":    {88, 4, "uint32"},
		"localeRegion":   {94, 1, "uint32"},
		"localeLanguage": {95, 1, "uint32"},
		"resourceStart":  {108, 4, "uint32"},
		"huffcdic":       {112, 4, "uint32"},
		"numHuffcdic":    {116, 4, "uint32"},
		"exthFlag":       {128, 4, "uint32"},
		"trailingFlags":  {240, 4, "uint32"},
		"indx":           {244, 4, "uint32"},
	}

	// KF8_HEADER_OFFSET_MAP KF8 头部字段偏移量映射
	KF8_HEADER_OFFSET_MAP = map[string][]interface{}{
		"resourceStart": {108, 4, "uint32"},
		"fdst":          {192, 4, "uint32"},
		"numFdst":       {196, 4, "uint32"},
		"frag":          {248, 4, "uint32"},
		"skel":          {252, 4, "uint32"},
		"guide":         {260, 4, "uint32"},
	}

	// EXTH_HEADER_OFFSET_MAP EXTH 头部字段偏移量映射
	EXTH_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":  {0, 4, "string"},
		"length": {4, 4, "uint32"},
		"count":  {8, 4, "uint32"},
	}

	// INDX_HEADER_OFFSET_MAP 索引头部字段偏移量映射
	INDX_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":      {0, 4, "string"},
		"length":     {4, 4, "uint32"},
		"type":       {8, 4, "uint32"},
		"idxt":       {20, 4, "uint32"},
		"numRecords": {24, 4, "uint32"},
		"encoding":   {28, 4, "uint32"},
		"language":   {32, 4, "uint32"},
		"total":      {36, 4, "uint32"},
		"ordt":       {40, 4, "uint32"},
		"ligt":       {44, 4, "uint32"},
		"numLigt":    {48, 4, "uint32"},
		"numCncx":    {52, 4, "uint32"},
	}

	// TAGX_HEADER_MAP TAGX 头部字段偏移量映射
	TAGX_HEADER_MAP = map[string][]interface{}{
		"magic":           {0, 4, "string"},
		"length":          {4, 4, "uint32"},
		"numControlBytes": {8, 4, "uint32"},
	}

	// HUFF_HEADER_MAP HUFF 头部字段偏移量映射
	HUFF_HEADER_MAP = map[string][]interface{}{
		"magic":   {0, 4, "string"},
		"offset1": {8, 4, "uint32"},
		"offset2": {12, 4, "uint32"},
	}

	// CDIC_HEADER_OFFSET_MAP CDIC 头部字段偏移量映射
	CDIC_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":      {0, 4, "string"},
		"length":     {4, 4, "uint32"},
		"numEntries": {8, 4, "uint32"},
		"codeLength": {12, 4, "uint32"},
	}

	// FDST_HEADER_OFFSET_MAP FDST 头部字段偏移量映射
	FDST_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":      {0, 4, "string"},
		"numEntries": {8, 4, "uint32"},
	}

	// FONT_HEADER_OFFSET_MAP FONT 头部字段偏移量映射
	FONT_HEADER_OFFSET_MAP = map[string][]interface{}{
		"flags":     {8, 4, "uint32"},
		"dataStart": {12, 4, "uint32"},
		"keyLength": {16, 4, "uint32"},
		"keyStart":  {20, 4, "uint32"},
	}

	// MOBI_ENCODING 编码方式映射表
	MOBI_ENCODING = map[uint32]string{
		1252:  "windows-1252",
		65001: "utf-8",
	}

	// EXTH_RECORD_TYPE EXTH 记录类型定义
	// 格式: 类型ID: {字段名, 数据类型, 是否为数组}
	EXTH_RECORD_TYPE = map[uint32][]interface{}{
		100: {"creator", "string", true},
		101: {"publisher"},
		103: {"description"},
		104: {"isbn"},
		105: {"subject", "string", true},
		106: {"date"},
		108: {"contributor", "string", true},
		109: {"rights"},
		110: {"subjectCode", "string", true},
		112: {"source", "string", true},
		113: {"asin"},
		121: {"boundary", "uint32"},
		122: {"fixedLayout"},
		125: {"numResources", "uint32"},
		126: {"originalResolution"},
		127: {"zeroGutter"},
		128: {"zeroMargin"},
		129: {"coverURI"},
		132: {"regionMagnification"},
		201: {"coverOffset", "uint32"},
		202: {"thumbnailOffset", "uint32"},
		503: {"title"},
		524: {"language", "string", true},
		527: {"pageProgressionDirection"},
	}

	// MOBI_LANG 语言代码映射表 (基于 Windows LCID)
	MOBI_LANG = map[uint32][]interface{}{
		1:  {"ar", "ar-SA", "ar-IQ", "ar-EG", "ar-LY", "ar-DZ", "ar-MA", "ar-TN", "ar-OM", "ar-YE", "ar-SY", "ar-JO", "ar-LB", "ar-KW", "ar-AE", "ar-BH", "ar-QA"},
		2:  {"bg"},
		3:  {"ca"},
		4:  {"zh", "zh-TW", "zh-CN", "zh-HK", "zh-SG"},
		5:  {"cs"},
		6:  {"da"},
		7:  {"de", "de-DE", "de-CH", "de-AT", "de-LU", "de-LI"},
		8:  {"el"},
		9:  {"en", "en-US", "en-GB", "en-AU", "en-CA", "en-NZ", "en-IE", "en-ZA", "en-JM", nil, "en-BZ", "en-TT", "en-ZW", "en-PH"},
		10: {"es", "es-ES", "es-MX", nil, "es-GT", "es-CR", "es-PA", "es-DO", "es-VE", "es-CO", "es-PE", "es-AR", "es-EC", "es-CL", "es-UY", "es-PY", "es-BO", "es-SV", "es-HN", "es-NI", "es-PR"},
		11: {"fi"},
		12: {"fr", "fr-FR", "fr-BE", "fr-CA", "fr-CH", "fr-LU", "fr-MC"},
		13: {"he"},
		14: {"hu"},
		15: {"is"},
		16: {"it", "it-IT", "it-CH"},
		17: {"ja"},
		18: {"ko"},
		19: {"nl", "nl-NL", "nl-BE"},
		20: {"no", "nb", "nn"},
		21: {"pl"},
		22: {"pt", "pt-BR", "pt-PT"},
		23: {"rm"},
		24: {"ro"},
		25: {"ru"},
		26: {"hr", nil, "sr"},
		27: {"sk"},
		28: {"sq"},
		29: {"sv", "sv-SE", "sv-FI"},
		30: {"th"},
		31: {"tr"},
		32: {"ur"},
		33: {"id"},
		34: {"uk"},
		35: {"be"},
		36: {"sl"},
		37: {"et"},
		38: {"lv"},
		39: {"lt"},
		41: {"fa"},
		42: {"vi"},
		43: {"hy"},
		44: {"az"},
		45: {"eu"},
		46: {"hsb"},
		47: {"mk"},
		48: {"st"},
		49: {"ts"},
		50: {"tn"},
		52: {"xh"},
		53: {"zu"},
		54: {"af"},
		55: {"ka"},
		56: {"fo"},
		57: {"hi"},
		58: {"mt"},
		59: {"se"},
		62: {"ms"},
		63: {"kk"},
		65: {"sw"},
		67: {"uz", nil, "uz-UZ"},
		68: {"tt"},
		69: {"bn"},
		70: {"pa"},
		71: {"or"},
		72: {"ta"},
		73: {"te"},
		74: {"kn"},
		75: {"ml"},
		76: {"as"},
		77: {"mr"},
		78: {"sa"},
		82: {"cy", "cy-GB"},
		83: {"gl", "gl-ES"},
		87: {"kok"},
		97: {"ne"},
		98: {"fy"},
	}
)
