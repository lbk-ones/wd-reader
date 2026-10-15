import {memo} from 'react';
import {isEmpty} from 'lodash-es';
import {Application, Window} from '@wailsio/runtime';
import {
    BookOutlined, BorderInnerOutlined, BorderOutlined, CloseOutlined,
    DatabaseOutlined, DisconnectOutlined, MinusOutlined, PushpinOutlined,
    ExpandOutlined, HolderOutlined, RedoOutlined, SettingOutlined,
} from '@ant-design/icons';
import {setCacheItem} from '../Utils.jsx';

function Header(props) {
    const {state, settingstate: settings} = props;
    const reading = !isEmpty(state.currentBookName);
    if (!props.display) return null;
    if (reading && settings.miniMode === '1') {
        return <aside className="mini-controls" aria-label="迷你阅读操作栏"
                      onClick={event => event.stopPropagation()}>
            <div className="mini-drag-handle" aria-label="拖动迷你窗口" title="按住此处拖动窗口"
                 style={{'--wails-draggable': 'drag'}}>
                <HolderOutlined aria-hidden="true"/>
            </div>
            <button type="button" className="mini-control-button" aria-label="阅读设置" title="阅读设置"
                    onClick={() => props.setState({settingVisible: true})}><SettingOutlined/></button>
            <button type="button" className="mini-control-button" aria-label="退出迷你模式" title="退出迷你模式（Esc）"
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
             title={reading ? state.currentBookName : props.title}
             onDoubleClick={event => {event.stopPropagation(); Window.ToggleMaximise()}}>
            <span className="brand-mark" aria-hidden="true"><BookOutlined/></span>
            <span className="title-name">{reading ? state.currentBookName.replace(/\.(txt|epub)$/i, '') : props.title}</span>
        </div>
        <nav className="window-controls" aria-label="窗口控制">
            {reading && control('隐藏标题栏', <BorderInnerOutlined/>, props.toggleTitle)}
            {reading && control('阅读设置', <SettingOutlined/>, () => props.setState({settingVisible: true}))}
            {reading && control('返回书架', <DatabaseOutlined/>, props.backBookList)}
            {control('离开窗口时隐藏', <DisconnectOutlined/>, () => toggleSetting('leaveWindowHid'), settings.leaveWindowHid === '1')}
            {control('窗口置顶', <PushpinOutlined/>, () => toggleSetting('isAlwaysTop'), settings.isAlwaysTop === '1')}
            {!reading && control('刷新书架', <RedoOutlined/>, () => Window.ForceReload())}
            <span className="window-controls-divider"/>
            {control('最小化', <MinusOutlined/>, () => Window.Minimise())}
            {control('切换窗口大小', <BorderOutlined/>, resizeWindow)}
            {control('关闭应用', <CloseOutlined/>, () => Application.Quit(), false, 'window-button--close')}
        </nav>
    </header>;
}

export default memo(Header);
