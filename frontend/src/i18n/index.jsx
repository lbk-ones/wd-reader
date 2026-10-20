import {useEffect, useSyncExternalStore} from 'react';
import {App as AntdApp, ConfigProvider} from 'antd';
import zhCN from 'antd/es/locale/zh_CN';
import enUS from 'antd/es/locale/en_US';
import {createLanguageStore, LANGUAGE_KEY, translate, translateBackendMessage} from './core.js';

const listeners = new Set();
let storage;
try { storage = window.localStorage; } catch { /* Private or restricted WebViews. */ }
const store = createLanguageStore({storage, browserLanguage: navigator.language,
    onChange: () => listeners.forEach(listener => listener())});
const subscribe = listener => {
    listeners.add(listener);
    return () => listeners.delete(listener);
};
window.addEventListener('storage', event => {
    if (event.key === LANGUAGE_KEY || event.key === null) store.sync();
});

// Read the active locale at invocation time, including callbacks registered at startup.
export const t = (source, values) => translate(store.getLanguage(), source, values);
export const localizeError = source => translateBackendMessage(store.getLanguage(), source);
export function useI18n() {
    const language = useSyncExternalStore(subscribe, store.getLanguage);
    return {language, setLanguage: store.setLanguage, t};
}

export function I18nProvider({children, theme}) {
    const {language} = useI18n();
    useEffect(() => { document.documentElement.lang = language; }, [language]);
    return <ConfigProvider locale={language === 'en' ? enUS : zhCN} theme={theme}>
        <AntdApp component={false}>{children}</AntdApp>
    </ConfigProvider>;
}
