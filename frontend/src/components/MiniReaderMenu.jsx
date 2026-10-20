import {useEffect, useRef, useState} from 'react';
import {Events} from '@wailsio/runtime';
import {
    ArrowLeftOutlined, ArrowRightOutlined, BookOutlined, BorderInnerOutlined,
    CloseOutlined, DatabaseOutlined, ExpandOutlined, LeftOutlined, LogoutOutlined,
    MinusOutlined, QuestionCircleOutlined, ReadOutlined, RightOutlined, SettingOutlined,
} from '@ant-design/icons';
import {CloseMiniMenu, GetMiniMenuState, SelectMiniMenuAction} from '../../bindings/wd-reader/app.js';
import {useI18n} from '../i18n/index.jsx';
import {MINI_MENU_GROUPS} from './readerMenu.js';
import './MiniReaderMenu.css';

const icons = {
    'previous-page': LeftOutlined, 'next-page': RightOutlined,
    'previous-chapter': ArrowLeftOutlined, 'next-chapter': ArrowRightOutlined,
    contents: BookOutlined, preferences: SettingOutlined, 'title-bar': BorderInnerOutlined,
    'exit-mini': ExpandOutlined, library: DatabaseOutlined, help: QuestionCircleOutlined,
    minimize: MinusOutlined, quit: LogoutOutlined,
};

export default function MiniReaderMenu() {
    const {setLanguage, t} = useI18n();
    const [context, setContext] = useState({bookName: '', chapterName: ''});
    const [pending, setPending] = useState(false);
    const [error, setError] = useState('');
    const firstAction = useRef(null);
    const selecting = useRef(false);

    async function selectAction(id) {
        if (selecting.current) return;
        selecting.current = true;
        setPending(true);
        setError('');
        try { await SelectMiniMenuAction(id); }
        catch (err) { setError(String(err)); }
        finally { selecting.current = false; setPending(false); }
    }

    useEffect(() => {
        let active = true;
        const update = data => {
            if (!active) return;
            setContext(data);
            if (data.language) setLanguage(data.language);
            setError('');
            requestAnimationFrame(() => firstAction.current?.focus());
        };
        const off = Events.On('mini-menu-context', event => update(event.data));
        GetMiniMenuState().then(update).catch(err => active && setError(String(err)));
        return () => { active = false; off(); };
    }, [setLanguage]);

    useEffect(() => {
        const onKey = event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                CloseMiniMenu().catch(err => setError(String(err)));
            } else if (!event.repeat && !event.ctrlKey && !event.altKey && !event.metaKey && !event.shiftKey) {
                const action = MINI_MENU_GROUPS.flatMap(group => group.actions).find(item => item.shortcut === event.key);
                if (action) { event.preventDefault(); selectAction(action.id); }
            }
        };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, []);

    return <main className="mini-reader-menu" aria-label={t('阅读操作')}>
        <header className="mini-menu-header">
            <div className="mini-menu-mark"><ReadOutlined/></div>
            <div className="mini-menu-heading">
                <span>{t('迷你阅读')}</span>
                <h1>{t('阅读操作')}</h1>
            </div>
            <button className="mini-menu-close" aria-label={t('关闭操作窗口')}
                    onClick={() => CloseMiniMenu().catch(err => setError(String(err)))}><CloseOutlined/></button>
        </header>
        <div className="mini-menu-book" title={`${context.bookName}\n${context.chapterName}`}>
            <BookOutlined/>
            <div><strong>{context.bookName || t('迷你阅读')}</strong><span>{context.chapterName || t('选择一项操作继续阅读')}</span></div>
        </div>
        <div className="mini-menu-groups">
            {MINI_MENU_GROUPS.map((group, groupIndex) => <section key={group.label}>
                <h2>{t(group.label)}</h2>
                <div className="mini-menu-action-grid">
                    {group.actions.map((action, actionIndex) => {
                        const Icon = icons[action.id];
                        return <button key={action.id} type="button" disabled={pending}
                                       ref={groupIndex === 0 && actionIndex === 0 ? firstAction : undefined}
                                       className={`mini-menu-action${action.danger ? ' mini-menu-action--danger' : ''}`}
                                       onClick={() => selectAction(action.id)}>
                            <Icon/><span>{t(action.label)}</span>{action.shortcut && <kbd>{action.shortcut}</kbd>}
                        </button>;
                    })}
                </div>
            </section>)}
        </div>
        <footer className="mini-menu-footer"><kbd>Esc</kbd><span>{t('关闭后继续阅读')}</span></footer>
        {error && <p role="alert" className="mini-menu-error">{error}</p>}
    </main>;
}
