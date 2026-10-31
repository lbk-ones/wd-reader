import {t, useI18n} from '../../i18n/index.jsx';
import {isEmpty} from "lodash-es";
import {
    DeleteOutlined,
    BookOutlined,
    DownloadOutlined,
    GithubOutlined,
    PlusOutlined,
    SettingOutlined,
    QuestionCircleOutlined,
    AppstoreOutlined,
    UnorderedListOutlined,
    EditOutlined,
    FolderOpenOutlined,
    ImportOutlined
} from "@ant-design/icons";
import {Button, Drawer, Input, Modal, message} from "antd";
import {getCacheItem, setCacheItem, getSplitMap, putSplitMap} from "../Utils";
import classNames from "classnames";
import {DeleteFile, GetScOne} from "../../../bindings/wd-reader/app";
import {forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState} from "react";
import {Browser, Call} from "@wailsio/runtime";
import {getCurrentlyReadingBook, sortBooksByLastOpened} from '../bookSorting.js';
import DefaultBookCover from '../DefaultBookCover.jsx';
import {indexOriginalName, matchesOriginalName} from '../librarySearch.js';

const TIP = ["翻开一页，暂别喧嚣。", "每一段闲暇，都有书相伴。", "慢慢读，世界会安静下来。"]

/**
 * book name list
 * @param props
 * @returns {JSX.Element}
 * @constructor
 */
function BookList(props, ref) {
    const {language} = useI18n();

    const state = props.state;

    const [sc, setSc] = useState("")
    const [libraryLayout, setLibraryLayout] = useState(() => getCacheItem('libraryLayout') === 'grid' ? 'grid' : 'list');
    const [metadata, setMetadata] = useState({});
    const [covers, setCovers] = useState({});
    const metadataRequestRef = useRef(0);
    const [editing, setEditing] = useState(null);
    const [searchMatches, setSearchMatches] = useState(null);
    const [libraryOpen, setLibraryOpen] = useState(false);
    const [originals, setOriginals] = useState([]);
    const [libraryQuery, setLibraryQuery] = useState('');
    const [missingOpen, setMissingOpen] = useState(false);
    const [missingBooks, setMissingBooks] = useState([]);
    const indexedOriginals = useMemo(() => originals.map(item => ({item, index: indexOriginalName(item.name)})), [originals]);
    const visibleOriginals = useMemo(() => indexedOriginals.filter(({index}) => matchesOriginalName(index, libraryQuery))
        .map(({item}) => item), [indexedOriginals, libraryQuery]);

    async function loadOriginals() {
        try { setOriginals(await Call.ByName('main.App.ListOriginalBooks')); }
        catch (error) { message.error(String(error)); }
    }
    async function loadMissing() {
        try { setMissingBooks(await Call.ByName('main.App.ListMissingBooks')); }
        catch (error) { message.error(String(error)); }
    }
    async function relinkMissing(item, name) {
        try {
            await Call.ByName('main.App.RelinkMissingBook', item.id, name);
            for (const prefix of ['', 'scroll-top']) {
                const value = getCacheItem(prefix + item.fileName);
                if (value !== null && getCacheItem(prefix + name) === null) setCacheItem(prefix + name, value);
            }
            const split = getSplitMap();
            for (const suffix of ['', '-Value']) {
                if (split[item.fileName + suffix] !== undefined && split[name + suffix] === undefined)
                    putSplitMap(name + suffix, split[item.fileName + suffix]);
            }
            props.reloadBookList(); await loadMissing(); await loadOriginals();
            message.success(t('已关联书籍文件'));
        } catch (error) { message.error(String(error)); }
    }
    async function restoreOriginal(item) {
        try {
            await Call.ByName('main.App.RestoreOriginalBook', item.name, item.inShelf);
            props.reloadBookList();
            await loadOriginals();
            message.success(t('已重新导入书架'));
        } catch (error) { message.error(String(error)); }
    }

    async function loadMetadata() {
        const request = ++metadataRequestRef.current;
        try {
            const books = await Call.ByName('main.App.GetBookMetadata');
            if (request !== metadataRequestRef.current) return books;
            props.onMetadataLoaded?.(books);
            setMetadata(Object.fromEntries(books.map(book => [book.fileName, book])));
            const images = await Promise.all(books.filter(book => book.cover).map(async book => {
                try { return [book.id, await Call.ByName('main.App.GetBookCover', book.id)]; }
                catch { return [book.id, '']; }
            }));
            if (request === metadataRequestRef.current) setCovers(Object.fromEntries(images));
            return books;
        } catch (error) { message.error(String(error)); return []; }
    }
    useEffect(() => { loadMetadata(); }, [state.currentBookList]);

    function changeLayout(layout) {
        setLibraryLayout(layout);
        setCacheItem('libraryLayout', layout);
    }

    useEffect(function () {
        GetScOne().then(res => {
            if (res) {
                try {
                    let parse = JSON.parse(res);
                    setSc(parse.hitokoto || TIP[parseInt(Math.random() * (TIP.length - 1))])
                } catch (e) {

                }
            } else {
                setSc(res || TIP[parseInt(Math.random() * (TIP.length - 1))])
            }
        })
    }, [])

    function selectBookFun(tem) {
        props.setState({
            loadingBook: true
        })
        let item = getCacheItem(tem);
        props.goChapterByName(tem, item, () => {
            props.clickBookToFirst(tem);
            Call.ByName('main.App.RecordBookOpened', tem).then(loadMetadata).catch(error => message.error(String(error)));
            props.beginRecordTop(tem)
            props.setState({loadingBook: false})
            props.calculateFontLines()
        });
    }

    useImperativeHandle(ref, () => ({
        selectBook: (tem) => {
            if (tem) {
                selectBookFun(tem)
            }
        },
        editImportedBook: async (name) => {
            const books = await loadMetadata();
            const book = books.find(item => item.fileName === name);
            if (book) setEditing({...book, authorsText: (book.authors || []).join('、')});
            else message.error(t('无法读取导入书籍的信息'));
        },
    }))

    const allBooks = sortBooksByLastOpened((state.currentBookList || []).filter(Boolean), metadata);
    const books = searchMatches === null ? allBooks : allBooks.filter(name => searchMatches.includes(name));
    const currentlyReading = getCurrentlyReadingBook(allBooks, metadata);

    return isEmpty(state.currentBookChapterName) && props.display && isEmpty(state.errorInfo) ? (
        <section className="book-list" aria-label={t("我的书架")}>
            <div className="library-heading">
                <div className="library-heading-row">
                    <h1>{t("我的书架")}<span className="library-count">{allBooks.length}</span></h1>
                    <span className="library-version">v{state.version || 'dev'}</span>
                    <div className="library-actions">
                        <Button type="primary" size="small" icon={<PlusOutlined/>}
                                aria-label={t("添加书籍")} title={t("添加书籍")} onClick={props.clickBookPlus}>
                            <span className="library-add-label">{t("添加书籍")}</span>
                        </Button>
                        <Button className="library-originals-button" size="small" icon={<FolderOpenOutlined/>}
                                aria-label={t('我的书库')} title={t('我的书库')}
                                onClick={() => { setLibraryOpen(true); loadOriginals(); }}><span className="library-originals-label">{t('我的书库')}</span></Button>
                        <Button className="library-download-button" size="small" icon={<DownloadOutlined/>} aria-label={t("图书下载")}
                                title={t("图书下载")} onClick={() => props.onSelect({id: 'download'})}>
                            <span className="library-download-label">{t("图书下载")}</span>
                        </Button>
                    </div>
                </div>
                <div className="library-toolbar">
                    <Input.Search className="library-search" allowClear placeholder={t("搜索书名、作者或拼音首字母")}
                                  aria-label={t("搜索书籍")} onSearch={value => {
                                      const query = value.trim().toLowerCase();
                                      const matches = query ? Object.values(metadata).filter(book =>
                                          [book.title, ...(book.authors || [])].some(field => field?.toLowerCase().includes(query)))
                                          .map(book => book.fileName) : [];
                                      setSearchMatches(query ? props.onSearch(value, matches) : null);
                                  }}/>
                    <div className="library-layout-switch" role="group" aria-label={t("书架布局")}>
                        <button type="button" aria-label={t("列表布局")} aria-pressed={libraryLayout === 'list'}
                                title={t("切换为列表布局")} onClick={() => changeLayout('list')}>
                            <UnorderedListOutlined/>
                        </button>
                        <button type="button" aria-label={t("矩阵布局")} aria-pressed={libraryLayout === 'grid'}
                                title={t("切换为矩阵布局")} onClick={() => changeLayout('grid')}>
                            <AppstoreOutlined/>
                        </button>
                    </div>
                </div>
            </div>
            <ul className={classNames('book-list-ul', {'book-list-ul--grid': libraryLayout === 'grid'})} aria-label={t("书籍列表")}>
                {books.map((name, index) => {
                    const meta = metadata[name];
                    const title = meta?.title || name.replace(/\.(txt|epub)$/i, '');
                    const isRecent = currentlyReading === name;
                    return <li key={name} className={classNames('library-book', {'library-book--recent': isRecent})}>
                        <button className="library-book-open" title={t("双击阅读：") + title}
                                onDoubleClick={() => selectBookFun(name)}
                                onKeyDown={event => {if (event.key === 'Enter') selectBookFun(name)}}>
                            <span className={'book-spine book-spine--' + index % 4} aria-hidden="true">
                                {covers[meta?.id] ? <img className="book-cover" src={covers[meta.id]} alt=""/> : <DefaultBookCover title={title}/>}
                            </span>
                            <span className="library-book-info">
                                <span className="library-book-title">{title}</span>
                                <span className="library-book-meta">
                                    <span>{(meta?.importType || name.split('.').pop()).toUpperCase()}</span>
                                    {meta?.authors?.length > 0 && <><span className="meta-dot">·</span><span className="library-book-author" title={meta.authors.join('、')}>{meta.authors.join('、')}</span></>}
                                    <span className="meta-dot">·</span>
                                    <span className="library-book-hint">{isRecent ? t("上次读到这里") : t("双击开始阅读")}</span>
                                </span>
                                {meta?.importedAt && <time className="library-book-import-date" dateTime={meta.importedAt} title={t('导入时间')}>
                                    {t('导入')} {new Date(meta.importedAt).toLocaleDateString(language === 'en' ? 'en' : 'zh-CN')}
                                </time>}
                            </span>
                            {isRecent && <span className="reading-badge">{t("在读")}</span>}
                        </button>
                        {meta && <button className="book-edit" aria-label={t('编辑书籍信息') + ' ' + title} title={t('编辑书籍信息')}
                                         onClick={() => setEditing({...meta, authorsText: (meta.authors || []).join('、')})}><EditOutlined/></button>}
                        <button className="book-delete" aria-label={t("删除 ") + title} title={t("删除书籍")}
                                onClick={() => DeleteFile(name).then(res => {
                                    if (!props.hasError(res)) {
                                        message.info(t("删除成功！"));
                                        props.reloadBookList();
                                    }
                                })}><DeleteOutlined/></button>
                    </li>
                })}
                {!books.length && searchMatches !== null && <li className="library-empty">{t('没有找到匹配的书籍')}</li>}
                {!allBooks.length && searchMatches === null && <li className="library-empty">
                    <BookOutlined/>
                    <h2>{t("留一处安静的书架")}</h2>
                    <p>{t("添加一本书，或将文件拖入窗口。")}</p>
                    <Button onClick={props.clickBookPlus}>{t("添加第一本书")}</Button>
                </li>}
            </ul>
            <Drawer className="original-library-drawer" title={t('我的书库')} open={libraryOpen} onClose={() => { setLibraryOpen(false); setLibraryQuery(''); }} width="min(420px, 100vw)">
                <p className="original-library-description">{t('这里保存导入时的原文件，可重新导入书架。')}</p>
                <Button className="missing-library-open" size="small" onClick={() => { setMissingOpen(true); loadMissing(); }}>{t('管理缺失书籍')}</Button>
                <Input.Search className="original-library-search" allowClear value={libraryQuery}
                              aria-label={t('搜索文件名、首拼或全拼')} placeholder={t('搜索文件名、首拼或全拼')}
                              onChange={event => setLibraryQuery(event.target.value)}/>
                <ul className="original-library-list">
                    {visibleOriginals.map(item => <li key={item.name} className="original-library-item">
                        <span className="original-library-name" title={item.name}>{item.name}</span>
                        <span className="original-library-actions">
                        <Button size="small" icon={<ImportOutlined/>} aria-label={t('重新导入')} title={t('重新导入')} onClick={() => item.inShelf ? Modal.confirm({
                            title: t('书架已有此书'), content: t('重新导入将替换书架中的解析文件，是否继续？'),
                            okText: t('我确定'), cancelText: t('取消'), onOk: () => restoreOriginal(item)
                        }) : restoreOriginal(item)}><span className="original-library-action-label">{t('重新导入')}</span></Button>
                        <Button size="small" danger icon={<DeleteOutlined/>} aria-label={t('删除原文件')} title={t('删除原文件')} onClick={() => Modal.confirm({
                            title: t('删除原文件'), content: t('删除后将无法从书库恢复此原文件，书架中的书不会删除。'),
                            okText: t('删除'), cancelText: t('取消'), onOk: async () => {
                                try { await Call.ByName('main.App.DeleteOriginalBook', item.name); await loadOriginals(); }
                                catch (error) { message.error(String(error)); }
                            }
                        })}><span className="original-library-action-label">{t('删除')}</span></Button>
                        </span>
                    </li>)}
                    {!originals.length && <li className="library-empty">{t('书库暂无原文件')}</li>}
                    {originals.length > 0 && !visibleOriginals.length && <li className="library-empty">{t('没有找到匹配的书籍')}</li>}
                </ul>
            </Drawer>
            <Drawer className="original-library-drawer" title={t('管理缺失书籍')} open={missingOpen}
                    onClose={() => setMissingOpen(false)} width="min(420px, 100vw)">
                <p className="original-library-description">{t('以下记录的阅读文件已缺失。可从原件恢复，或关联内容相同的文件。')}</p>
                <ul className="missing-library-list">
                    {missingBooks.map(item => <li className="missing-library-item" key={item.id}>
                        <div className="missing-library-title" title={item.title}>{item.title || item.fileName}</div>
                        <div className="missing-library-filename" title={item.fileName}>{item.fileName}</div>
                        <div className="missing-library-actions">
                            {item.canRestore && <Button size="small" onClick={async () => {
                                try { await Call.ByName('main.App.RestoreOriginalBook', item.originalName, false); props.reloadBookList(); await loadMissing(); await loadOriginals(); message.success(t('已重新导入书架')); }
                                catch (error) { message.error(String(error)); }
                            }}>{t('从原件恢复')}</Button>}
                            <Button size="small" danger onClick={() => Modal.confirm({title: t('删除缺失记录'),
                                content: t('将删除该书的元信息和封面，不删除原文件。'), okText: t('删除'), cancelText: t('取消'),
                                onOk: async () => { try { await Call.ByName('main.App.DeleteMissingBook', item.id); await loadMissing(); } catch (error) { message.error(String(error)); } }
                            })}>{t('删除记录')}</Button>
                        </div>
                        {item.candidates.length > 0 && <div className="missing-library-candidates">
                            <span>{t('内容相同的候选文件')}</span>
                            {item.candidates.map(name => <div className="missing-library-candidate" key={name}>
                                <span title={name}>{name}</span><Button size="small" onClick={() => Modal.confirm({
                                    title: t('关联书籍文件'), content: t('将此缺失记录关联到 {0}？', {0: name}),
                                    okText: t('我确定'), cancelText: t('取消'), onOk: () => relinkMissing(item, name)
                                })}>{t('关联')}</Button>
                            </div>)}
                        </div>}
                    </li>)}
                    {!missingBooks.length && <li className="library-empty">{t('没有缺失的书籍')}</li>}
                </ul>
            </Drawer>
            <Modal title={t('编辑书籍信息')} open={!!editing} okText={t('保存')} cancelText={t('取消')}
                   onCancel={() => setEditing(null)} onOk={async () => {
                       try {
                           await Call.ByName('main.App.UpdateBookMetadata', editing.id, editing.title,
                               editing.authorsText.split(/[、,，]/).map(s => s.trim()).filter(Boolean), editing.description || '');
                           setEditing(null); await loadMetadata();
                       } catch (error) { message.error(String(error)); }
                   }}>
                {editing && <div className="book-metadata-form">
                    <label>{t('书名')}<Input value={editing.title} onChange={e => setEditing({...editing, title: e.target.value})}/></label>
                    <label>{t('作者')}<Input value={editing.authorsText} onChange={e => setEditing({...editing, authorsText: e.target.value})}/></label>
                    <label>{t('简介')}<Input.TextArea rows={3} value={editing.description || ''} onChange={e => setEditing({...editing, description: e.target.value})}/></label>
                    <div><Button onClick={async () => { try { if (await Call.ByName('main.App.ChooseBookCover', editing.id)) await loadMetadata(); } catch (error) { message.error(String(error)); } }}>{t('选择封面')}</Button>{' '}
                        <Button onClick={async () => { try { await Call.ByName('main.App.RemoveBookCover', editing.id); await loadMetadata(); } catch (error) { message.error(String(error)); } }}>{t('移除封面')}</Button></div>
                    <small>{t('封面修改立即保存')}</small>
                </div>}
            </Modal>
            <footer className="library-footer">
                <span className="library-quote" title={language === 'en' ? t(TIP[0]) : sc}>{language === 'en' ? t(TIP[0]) : (sc || t(TIP[0]))}</span>
                <button className="footer-icon" title={t("使用说明（F1）")} aria-label={t("使用说明")}
                        onClick={props.onOpenHelp}><QuestionCircleOutlined/></button>
                <button className="footer-icon" title={t("项目主页")} aria-label={t("项目主页")}
                        onClick={() => Browser.OpenURL('https://github.com/lbk-ones/wd-reader')}><GithubOutlined/></button>
                <button className="footer-icon" title={t("应用设置")} aria-label={t("应用设置")}
                        onClick={() => props.setState({sysSettingVisible: true})}><SettingOutlined/></button>
            </footer>
        </section>
    ) : null;
}

export default forwardRef(BookList)
