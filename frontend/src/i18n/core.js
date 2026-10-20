import en from './en.js';
import enHelp from './en-help.js';

export const LANGUAGE_KEY = 'Wd-language';
export const LANGUAGES = ['zh-CN', 'en'];

export function resolveLanguage(saved, browserLanguage = 'zh-CN') {
    if (LANGUAGES.includes(saved)) return saved;
    return /^en(?:-|$)/i.test(browserLanguage) ? 'en' : 'zh-CN';
}

// Chinese source strings are the default catalog and readable translation keys.
export function translate(language, source, values = {}) {
    const english = Object.hasOwn(en, source) ? en[source] : Object.hasOwn(enHelp, source) ? enHelp[source] : source;
    const text = language === 'en' ? english : source;
    return text.replace(/\{(\w+)\}/g, (match, key) => Object.hasOwn(values, key) ? String(values[key]) : match);
}

// Preserve the Go wire format; translate only known application errors for display.
export function translateBackendMessage(language, message) {
    const source = String(message);
    const prefix = '错误信息:';
    if (source.startsWith(prefix)) {
        return translate(language, prefix) + translateBackendMessage(language, source.slice(prefix.length));
    }
    for (const prefix of ['断章正则无效: ', '读取书籍失败: ']) {
        if (source.startsWith(prefix)) return translate(language, prefix) + source.slice(prefix.length);
    }
    const missingFile = source.match(/^未找到([.\w]+)文件$/);
    if (missingFile) return translate(language, '未找到{suffix}文件', {suffix: missingFile[1]});
    return translate(language, source);
}

export function createLanguageStore({storage, browserLanguage = 'zh-CN', onChange = () => {}}) {
    const read = () => {
        try { return storage?.getItem(LANGUAGE_KEY); } catch { return null; }
    };
    let language = resolveLanguage(read(), browserLanguage);
    return {
        getLanguage: () => language,
        setLanguage(value) {
            if (!LANGUAGES.includes(value)) return;
            try { storage?.setItem(LANGUAGE_KEY, value); } catch { /* Keep switching available without storage. */ }
            language = value;
            onChange();
        },
        sync() {
            language = resolveLanguage(read(), browserLanguage);
            onChange();
        },
    };
}
