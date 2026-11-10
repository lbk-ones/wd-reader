package main

import "strings"

var categories = map[string][]string{
	"都市":   {"都市小说", "都市青春", "都市娱乐"},
	"现实":   {"现实小说", "当代现代"},
	"乡村":   {"乡村小说"},
	"官场":   {"官场小说"},
	"历史军事": {"历史小说", "军事小说", "军事历史"},
	"武侠":   {"武侠小说", "传统武侠"},
	"仙侠":   {"仙侠小说", "修真仙侠", "仙侠奇缘"},
	"玄幻奇幻": {"玄幻小说", "奇幻小说", "仙侠奇幻"},
	"科幻":   {"科幻小说", "科幻空间", "科幻灵异"},
	"悬疑惊悚": {"侦探推理", "惊悚悬疑", "悬疑小说", "悬疑推理"},
	"游戏电竞": {"网游小说", "游戏小说", "网游竞技"},
	"体育":   {"体育小说"},
	"现言":   {"现代言情", "总裁豪门"},
	"古言":   {"古代言情", "台言古言", "古代架空"},
	"玄幻言情": {},
	"穿越":   {"穿越小说", "言情穿越"},
	"重生":   {"重生小说"},
	"无限流":  {"诸天无限", "无限流派"},
	"轻小说":  {},
	"同人":   {"同人小说"},
	"耽美":   {"耽于纯美", "耽美同人"},
	"无CP":  {"无CP向"},
	"名著古籍": {"文学名著"},
	"未分类":  {"其他"},
}

func categoryIndex() map[string]string {
	index := make(map[string]string)
	for category, aliases := range categories {
		index[category] = category
		for _, alias := range aliases {
			index[alias] = category
		}
	}
	return index
}

const (
	maleChannel    = "男频"
	femaleChannel  = "女频"
	unknownChannel = "未判定"
)

var maleTitleHints = []string{
	"赘婿", "贅婿", "兵王", "战神归来", "龙王归来", "仙帝归来", "都市修仙", "神豪", "万倍返还",
	"开局签到", "開局簽到", "开局觉醒", "开局获得", "开局融合", "开局无敌", "修仙从", "我在诸天",
	"横推诸天", "悟性逆天", "从斩妖",
}

var femaleTitleHints = []string{
	"快穿", "穿书", "女配", "团宠", "甜宠", "嫡女", "庶女", "王妃", "和离", "军嫂",
	"團寵", "萌宝", "追妻火葬场", "先婚后爱", "白月光", "真千金", "假千金", "恋综", "女尊", "双男主",
	"抄家流放", "带崽", "惡毒女配",
}

// normalizeChannel 只识别频道类型，保留 EPUB 中其他 dc:type 的用途。
func normalizeChannel(value string) string {
	switch strings.TrimSpace(value) {
	case maleChannel, "男頻":
		return maleChannel
	case femaleChannel, "女頻":
		return femaleChannel
	case unknownChannel:
		return unknownChannel
	default:
		return ""
	}
}

// inferChannel 优先使用显式频道标签，其次识别典型书名用语。通用题材词不单独参与判定。
func inferChannel(title, category, existing string) string {
	title = normalize(title)
	male := hasTitleTag(title, maleChannel) || hasTitleTag(title, "男頻")
	female := hasTitleTag(title, femaleChannel) || hasTitleTag(title, "女頻") ||
		hasTitleTag(title, "纯爱") || hasTitleTag(title, "百合")
	if !male && !female {
		male, female = hasTitleHint(title, maleTitleHints), hasTitleHint(title, femaleTitleHints)
	}
	if male && !female {
		return maleChannel
	}
	if female && !male {
		return femaleChannel
	}
	if existing == maleChannel || existing == femaleChannel {
		return existing
	}
	switch category {
	case "现言", "古言", "玄幻言情", "耽美":
		return femaleChannel
	default:
		return unknownChannel
	}
}

func hasTitleTag(title, tag string) bool {
	return strings.Contains(title, "["+tag+"]") || strings.Contains(title, "【"+tag+"】") ||
		strings.Contains(title, "("+tag+")")
}

func hasTitleHint(title string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(title, hint) {
			return true
		}
	}
	return false
}
