import pinyin from 'pinyin';

const normalize = value => value.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');

export function indexOriginalName(name) {
    const phonetic = style => normalize(pinyin(name, {style}).map(parts => parts[0]).join(''));
    return {name: name.toLowerCase(), normalized: normalize(name),
        initials: phonetic(pinyin.STYLE_FIRST_LETTER), full: phonetic(pinyin.STYLE_NORMAL)};
}

export function matchesOriginalName(index, query) {
    const raw = query.trim().toLowerCase();
    if (!raw) return true;
    const clean = normalize(raw);
    return index.name.includes(raw) || (!!clean && [index.normalized, index.initials, index.full]
        .some(value => value.includes(clean)));
}
