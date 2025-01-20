package mobi

type PALMDOC_HEADER struct {
	Compression    int `json:"compression"`
	NumTextRecords int `json:"numTextRecords"`
	RecordSize     int `json:"recordSize"`
	Encryption     int `json:"encryption"`
}
type MOBI_HEADER struct {
	Magic          string `json:"magic"`
	Length         uint32 `json:"length"`
	Type           uint32 `json:"type"`
	Encoding       uint32 `json:"encoding"`
	Uid            uint32 `json:"uid"`
	Version        uint32 `json:"version"`
	TitleOffset    uint32 `json:"titleOffset"`
	TitleLength    uint32 `json:"titleLength"`
	LocaleRegion   uint32 `json:"localeRegion"`
	LocaleLanguage uint32 `json:"localeLanguage"`
	ResourceStart  uint32 `json:"resourceStart"`
	Huffcdic       uint32 `json:"huffcdic"`
	NumHuffcdic    uint32 `json:"numHuffcdic"`
	ExthFlag       uint32 `json:"exthFlag"`
	TrailingFlags  uint32 `json:"trailingFlags"`
	Indx           uint32 `json:"indx"`
	Title          []byte `json:"title"`
	Language       string `json:"language"`
}

type EXTH_HEADER struct {
	Magic   string    `json:"magic"`
	Length  int       `json:"length"`
	Count   int       `json:"count"`
	ExthRow *EXTH_ROW `json:"exthRow"`
}
type EXTH_ROW struct {
	Creator                  []string `json:"creator"`
	Publisher                string   `json:"publisher"`
	Description              string   `json:"description"`
	ISBN                     string   `json:"isbn"`
	Subject                  []string `json:"subject"`
	Date                     string   `json:"date"`
	Contributor              []string `json:"contributor"`
	Rights                   string   `json:"rights"`
	SubjectCode              []string `json:"subjectCode"`
	Source                   []string `json:"source"`
	ASIN                     string   `json:"asin"`
	Boundary                 uint32   `json:"boundary"`
	FixedLayout              string   `json:"fixedLayout"`
	NumResources             uint32   `json:"numResources"`
	OriginalResolution       string   `json:"originalResolution"`
	ZeroGutter               string   `json:"zeroGutter"`
	ZeroMargin               string   `json:"zeroMargin"`
	CoverURI                 string   `json:"coverURI"`
	RegionMagnification      string   `json:"regionMagnification"`
	CoverOffset              uint32   `json:"coverOffset"`
	ThumbnailOffset          uint32   `json:"thumbnailOffset"`
	Title                    string   `json:"title"`
	Language                 []string `json:"language"`
	PageProgressionDirection string   `json:"pageProgressionDirection"`
}
type KF8_HEADER struct {
	ResourceStart int `json:"resourceStart"`
	Fdst          int `json:"fdst"`
	NumFdst       int `json:"numFdst"`
	Frag          int `json:"frag"`
	Skel          int `json:"skel"`
	Guide         int `json:"guide"`
}
type HEADERS struct {
	PalmdocHeader *PALMDOC_HEADER `json:"palmdoc"`
	PdbHeader     *PDB_HEADER     `json:"pdb"`
	MobiHeader    *MOBI_HEADER    `json:"mobi"`
	ExthHeader    *EXTH_HEADER    `json:"exth"`
	Kf8Header     *KF8_HEADER     `json:"kf8"`
}
type DecompressFunc func([]byte) ([]byte, error)
type RemoveTrailEntriesFunc func([]byte) []byte
type MOBI_BOOK struct {
	Headers            *HEADERS               `json:"headers"`
	IS_KF8             bool                   `json:"isKf8"`
	Decompress         DecompressFunc         `json:"-"`
	RemoveTrailEntries RemoveTrailEntriesFunc `json:"-"`
}

type PDB_HEADER struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Creator    string `json:"creator"`
	NumRecords int    `json:"numRecords"`
}

// HUFF_HEADER HUFF_HEADER 是单独纯在的 不纳入 HEADERS里面去
type HUFF_HEADER struct {
	Magic   string `json:"magic"`
	Offset1 uint32 `json:"offset1"`
	Offset2 uint32 `json:"offset2"`
}

// CDIC_HEADER CDIC_HEADER 是单独纯在的 不纳入 HEADERS里面去
type CDIC_HEADER struct {
	Magic      string `json:"magic"`
	Length     uint32 `json:"length"`
	NumEntries uint32 `json:"numEntries"`
	CodeLength uint32 `json:"codeLength"`
}

var (
	// MIME map
	MIME = map[string]string{
		"XML":   "application/xml",
		"XHTML": "application/xhtml+xml",
		"HTML":  "text/html",
		"CSS":   "text/css",
		"SVG":   "image/svg+xml",
	}

	// PDB_HEADER_OFFSET_MAP map
	PDB_HEADER_OFFSET_MAP = map[string][]interface{}{
		"name":       {0, 32, "string"},
		"type":       {60, 4, "string"},
		"creator":    {64, 4, "string"},
		"numRecords": {76, 2, "uint16"},
	}

	// PALMDOC_HEADER_OFFSET_MAP map
	PALMDOC_HEADER_OFFSET_MAP = map[string][]interface{}{
		"compression":    {0, 2, "uint16"},
		"numTextRecords": {8, 2, "uint16"},
		"recordSize":     {10, 2, "uint16"},
		"encryption":     {12, 2, "uint16"},
	}

	// MOBI_HEADER_OFFSET_MAP map
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

	// KF8_HEADER_OFFSET_MAP map
	KF8_HEADER_OFFSET_MAP = map[string][]interface{}{
		"resourceStart": {108, 4, "uint32"},
		"fdst":          {192, 4, "uint32"},
		"numFdst":       {196, 4, "uint32"},
		"frag":          {248, 4, "uint32"},
		"skel":          {252, 4, "uint32"},
		"guide":         {260, 4, "uint32"},
	}

	// EXTH_HEADER_OFFSET_MAP map
	EXTH_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":  {0, 4, "string"},
		"length": {4, 4, "uint32"},
		"count":  {8, 4, "uint32"},
	}

	// INDX_HEADER_OFFSET_MAP map
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

	// TAGX_HEADER_MAP map
	TAGX_HEADER_MAP = map[string][]interface{}{
		"magic":           {0, 4, "string"},
		"length":          {4, 4, "uint32"},
		"numControlBytes": {8, 4, "uint32"},
	}

	// HUFF_HEADER_MAP map
	HUFF_HEADER_MAP = map[string][]interface{}{
		"magic":   {0, 4, "string"},
		"offset1": {8, 4, "uint32"},
		"offset2": {12, 4, "uint32"},
	}

	// CDIC_HEADER_OFFSET_MAP map
	CDIC_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":      {0, 4, "string"},
		"length":     {4, 4, "uint32"},
		"numEntries": {8, 4, "uint32"},
		"codeLength": {12, 4, "uint32"},
	}

	// FDST_HEADER_OFFSET_MAP map
	FDST_HEADER_OFFSET_MAP = map[string][]interface{}{
		"magic":      {0, 4, "string"},
		"numEntries": {8, 4, "uint32"},
	}

	// FONT_HEADER_OFFSET_MAP map
	FONT_HEADER_OFFSET_MAP = map[string][]interface{}{
		"flags":     {8, 4, "uint32"},
		"dataStart": {12, 4, "uint32"},
		"keyLength": {16, 4, "uint32"},
		"keyStart":  {20, 4, "uint32"},
	}

	// MOBI_ENCODING map
	MOBI_ENCODING = map[uint32]string{
		1252:  "windows-1252",
		65001: "utf-8",
	}

	// EXTH_RECORD_TYPE map
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

	// MOBI_LANG map
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
