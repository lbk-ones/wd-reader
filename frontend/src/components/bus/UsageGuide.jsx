import {Drawer, Tabs} from 'antd';
import {ReadOutlined} from '@ant-design/icons';

function Section({title, children}) {
    return <section className="usage-guide-section"><h3>{title}</h3>{children}</section>;
}

const topics = [
    {key: 'reading', label: '开始阅读', children: <>
        <Section title="把书放进书架">
            <p>点击“添加书籍”选择文件，或把本地文件拖入窗口。优先使用 TXT、EPUB、MOBI、AZW、AZW3；其他格式需要联网解析转换，大文件可能需要稍等。</p>
            <p>“图书下载”支持输入文件链接，也提供图书资源入口。</p>
        </Section>
        <Section title="找到并打开一本书">
            <p>书架“藏书一览”右侧可以一键切换列表、矩阵布局。矩阵会随窗口宽度调整列数，布局偏好会自动保存。</p>
            <p>搜索支持书名和拼音首字母。双击书籍开始阅读，使用键盘选中书籍后按 Enter 也能打开。</p>
            <p>“在读”标记指向最近阅读的书。章节和阅读位置会自动记录，下次打开继续阅读。</p>
        </Section>
        <Section title="翻页与切换章节">
            <p>启用“点击翻页”后，单击正文翻到下一页；也可以使用鼠标滚轮。普通模式底部可切换上一章、下一章。</p>
            <p>滚轮可以自由滚动；随后使用上一页、下一页时会自动对齐文字行，避免页面上下边缘遮住半行字。</p>
            <p>自动翻页的间隔可在阅读偏好中设置。手动翻页或滚动正文后，会重新计时，让新页面保留完整的阅读时间。</p>
            <p>右键点击正文，可以打开目录、切换章节、向前或向后翻页，以及返回书架。目录支持搜索和选择章节。</p>
            <p>目录会自动过滤没有正文的章节，并合并标题和正文均重复的章节。同名但内容不同的章节会标注“同名”，短章节仍会保留；翻章和自动翻页使用同一份有效目录。</p>
        </Section>
        <Section title="标题栏隐藏后怎么办？">
            <p>右键点击正文，选择“标题开关”恢复标题栏，选择“设置”打开阅读偏好。使用说明也能通过右键菜单或 F1 再次打开。</p>
        </Section>
    </>},
    {key: 'preferences', label: '阅读偏好', children: <>
        <Section title="字号、行距与配色">
            <p>点击阅读界面的齿轮，调整字号、行间距、背景色和字体颜色。点击颜色旁的“更改”打开调色板，可选择预设色或输入色值。</p>
            <p>偏好会自动保存。“恢复默认”重置阅读偏好；仅退出迷你模式时，会恢复进入前的字号和行高。</p>
        </Section>
        <Section title="迷你阅读">
            <p>在阅读偏好中选择 1–4 行，再开启“迷你化”。右侧六点区域可以拖动窗口，齿轮打开偏好，展开图标一键退出。</p>
            <p>按 Esc 也能退出迷你模式。编辑偏好或查看目录时，此快捷键不会直接退出迷你模式。</p>
            <p>打开使用说明会先恢复普通窗口，便于查看内容。</p>
        </Section>
        <Section title="置顶、隐藏与透明">
            <p>“窗口置顶”让阅读窗口保持在其他窗口上方。“窗口隐藏”开启后，鼠标移出窗口会隐藏内容，移回即可显示。</p>
            <p>“透明模式”使用透明背景，并将文字改成白色。关闭后如需更换文字颜色，可在调色板中重新选择。</p>
        </Section>
        <Section title="章节识别不准确？">
            <p>在“断章方式”中选择智能识别、按段落数分章或自定义正则，再点击“应用断章方式”。应用时会提示清空该书的章节阅读历史。</p>
        </Section>
    </>},
    {key: 'shortcuts', label: '快捷操作', children: <>
        <Section title="常用快捷键">
            <dl className="usage-guide-shortcuts">
                <div><dt><kbd>F1</kbd></dt><dd>打开使用说明</dd></div>
                <div><dt><kbd>F2</kbd></dt><dd>打开 / 关闭目录</dd></div>
                <div><dt><kbd>F3</kbd> / <kbd>F4</kbd></dt><dd>上一页 / 下一页</dd></div>
                <div><dt><kbd>F5</kbd> / <kbd>F6</kbd></dt><dd>上一章 / 下一章</dd></div>
                <div><dt><kbd>F7</kbd></dt><dd>开启 / 关闭自动翻页（间隔在阅读偏好中设置）</dd></div>
                <div><dt><kbd>F8</kbd></dt><dd>进入 / 退出迷你模式（使用已保存的行数）</dd></div>
                <div><dt><kbd>Esc</kbd></dt><dd>关闭使用说明；在迷你阅读正文中退出迷你模式</dd></div>
                <div><dt><kbd>Enter</kbd></dt><dd>打开书架中已选中的书籍</dd></div>
                <div><dt><kbd>Ctrl / ⌘</kbd> + <kbd>Space</kbd></dt><dd>老板键：最小化窗口，再按恢复（Mac 使用 ⌘）</dd></div>
            </dl>
            <p className="usage-guide-note">F2–F8 在阅读正文时生效；编辑输入框、加载书籍或打开其他面板时暂停。目录打开时可按 F2 关闭；按住按键不会连续触发。</p>
            <p className="usage-guide-note">阅读时 F5 用于上一章；打开设置、目录或编辑输入框时，F5 不会切章，也不会刷新页面。</p>
            <p className="usage-guide-note">老板键需要成功注册系统快捷键；如与其他程序冲突，可使用最小化按钮。Linux 全局快捷键依赖 X11 环境。</p>
        </Section>
        <Section title="移动与调整窗口">
            <p>普通模式按住标题栏左侧的书名区域拖动窗口。顶部方框按钮切换窗口大小，减号按钮最小化。</p>
            <p>迷你模式按住右侧六点区域拖动，点击右下角展开图标退出，并恢复原窗口大小和阅读排版。</p>
        </Section>
        <Section title="记录与缓存">
            <p>阅读偏好和进度自动保存在本机。系统设置中的“清空所有缓存”会清除阅读记录和偏好，操作前请留意确认提示。</p>
            <p>“清除阅读记录”只删除最近阅读列表、章节记录和阅读位置，保留字体、配色、书架布局及断章偏好。</p>
        </Section>
    </>},
];

export default function UsageGuide({open, onClose}) {
    return <Drawer title={<span className="usage-guide-title"><ReadOutlined/> 使用说明</span>}
                   rootClassName="usage-guide" placement="right" width="min(100vw, 520px)"
                   open={open} onClose={onClose} destroyOnClose
                   closeIcon={<span aria-label="关闭使用说明">×</span>}>
        <p className="usage-guide-intro">从第一本书开始，找到适合自己的阅读方式。<span>内置说明 · 离线可读</span></p>
        <Tabs defaultActiveKey="reading" items={topics}/>
    </Drawer>;
}
