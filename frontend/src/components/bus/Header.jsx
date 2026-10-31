import {t, useI18n} from '../../i18n/index.jsx';
import {memo} from 'react';
import {isEmpty} from 'lodash-es';
import {Application, Window} from '@wailsio/runtime';
import {
    BookOutlined,
    BorderInnerOutlined,
    BorderOutlined,
    CloseOutlined,
    DatabaseOutlined,
    DisconnectOutlined,
    ExpandOutlined,
    HolderOutlined,
    MinusOutlined,
    PushpinOutlined,
    QuestionCircleOutlined,
    RedoOutlined,
    SettingOutlined,
} from '@ant-design/icons';
import {setCacheItem} from '../Utils.jsx';

function Header(props) {
    useI18n();
    const {state, settingstate: settings} = props;
    const reading = !isEmpty(state.currentBookName);
    const bookTitle = state.bookMetadata?.[state.currentBookName]?.title
        || state.currentBookName.replace(/\.(txt|epub)$/i, '');
    if (!props.display) return null;
    if (reading && settings.miniMode === '1') {
        return <aside className="mini-controls" aria-label={t("迷你阅读操作栏")}
                      onClick={event => event.stopPropagation()}>
            <div className="mini-drag-handle" aria-label={t("拖动迷你窗口")}
                 title={t("当前章节：{0}\n按住此处拖动窗口", {0: state.currentBookChapterName})}
                 style={{'--wails-draggable': 'drag'}}>
                <HolderOutlined aria-hidden="true"/>
            </div>
            <button type="button" className="mini-control-button" aria-label={t("阅读设置")} title={t("阅读设置")}
                    onClick={() => props.setState({settingVisible: true})}><SettingOutlined/></button>
            <button type="button" className="mini-control-button" aria-label={t("退出迷你模式")} title={t("退出迷你模式（Esc）")}
                    onClick={props.exitMiniMode}><ExpandOutlined/></button>
        </aside>;
    }
    if (!state.showTitle && reading) return null;

    function toggleSetting(key) {
        const value = settings[key] === '1' ? '0' : '1';
        setCacheItem(key, value);
        props.setSettingState({[key]: value});
        if (key === 'isAlwaysTop') props.isAlwaysTop(value);
    }

    async function resizeWindow() {
        const size = await Window.Size();
        await Window.SetSize(size.width === 400 && size.height === 690 ? 960 : 400,
            size.width === 400 && size.height === 690 ? 540 : 690);
        await Window.Center();
    }

    const control = (label, icon, onClick, active = false, extraClass = '') => (
        <button key={label} className={`window-button ${active ? 'is-active' : ''} ${extraClass}`}
                aria-label={label} title={label} onClick={onClick}>{icon}</button>
    );

    return <header className={`title ${reading ? 'title--reading' : 'title--library'}`}>
        <div className="title-brand" style={{'--wails-draggable': 'drag'}}
             title={reading ? t("{0}\n当前章节：{1}", {0: bookTitle, 1: state.currentBookChapterName}) : props.title}
             onDoubleClick={event => {
                 event.stopPropagation();
                 Window.ToggleMaximise()
             }}>
            <span className="brand-mark" aria-hidden="true"><BookOutlined/></span>
            <span
                className="title-name">{reading ? bookTitle : props.title}</span>
        </div>
        <nav className="window-controls" aria-label={t("窗口控制")}>
            {reading && control(t("隐藏标题栏"), <BorderInnerOutlined/>, props.toggleTitle)}
            {reading && control(t("阅读设置"), <SettingOutlined/>, () => props.setState({settingVisible: true}))}
            {reading && control(t("返回书架"), <DatabaseOutlined/>, () => props.backBookList())}
            {reading && control(t("使用说明"), <QuestionCircleOutlined/>, props.onOpenHelp)}
            {control(t("离开窗口时隐藏"),
                <DisconnectOutlined/>, () => toggleSetting('leaveWindowHid'), settings.leaveWindowHid === '1')}
            {control(t("窗口置顶"), <PushpinOutlined/>, () => toggleSetting('isAlwaysTop'), settings.isAlwaysTop === '1')}
            {!reading && control(t("刷新书架"), <RedoOutlined/>, () => window.location.reload())}
            <span className="window-controls-divider"/>
            {control(t("最小化"), <MinusOutlined/>, () => Window.Minimise())}
            {control(t("切换窗口大小"), <BorderOutlined/>, resizeWindow)}
            {control(t("关闭应用"), <CloseOutlined/>, () => Application.Quit(), false, 'window-button--close')}
        </nav>
    </header>;
}

export default memo(Header);
