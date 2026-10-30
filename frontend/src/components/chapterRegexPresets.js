const number = '[0-9零〇一二三四五六七八九十百千万两壹贰叁肆伍陆柒捌玖拾佰仟]+';
const prefix = `^第[ 　]*${number}[ 　]*`;

// These expressions use Go's regexp syntax, as matching happens in the backend.
export const CHAPTER_REGEX_PRESETS = [
    {label: '第几章', pattern: `${prefix}章`},
    {label: '第几章／节／回', pattern: `${prefix}[章节回]`},
    {label: '第几卷／部', pattern: `${prefix}[卷部]`},
    {label: '第几章/卷', pattern: `${prefix}[章卷]`},
    {label: '英文 Chapter 数字', pattern: '^[Cc][Hh][Aa][Pp][Tt][Ee][Rr][ 　]+[0-9]+'},
];
