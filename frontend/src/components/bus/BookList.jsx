import {t, useI18n, localizeError} from '../../i18n/index.jsx';
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
    UnorderedListOutlined
} from "@ant-design/icons";
import {Button, Input, message} from "antd";
import {getCacheItem, setCacheItem} from "../Utils";
import classNames from "classnames";
import {DeleteEpubFile, DeleteFile, GetScOne, ParseEpubToTxt} from "../../../bindings/wd-reader/app";
import {forwardRef, useEffect, useImperativeHandle, useState} from "react";
import {Browser} from "@wailsio/runtime";

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

    function sortBookList() {
        let currentBookList = state.currentBookList;

        if (!isEmpty(currentBookList)) {
            let item = getCacheItem("LastClickBook") || "";
            let first = [];
            let last = [];
            (item.split(",")).forEach(e => {
                if (currentBookList.indexOf(e) >= 0) {
                    first.push(e);
                }
            })
            currentBookList.forEach(e => {
                if (first.indexOf(e) < 0) {
                    last.push(e);
                }
            })
            return first.concat(last);
        }

        return currentBookList;
    }

    function selectBookFun(tem) {
        props.clickBookToFirst(tem);
        props.setState({
            loadingBook: true
        })
        if (tem.endsWith("epub")) {

            ParseEpubToTxt(tem).then(res => {
                if (res.startsWith("错误信息:")) {
                    message.error(localizeError(res))
                    props.setState({loadingBook: false});
                    return;
                }

                let s = tem.replace(".epub", ".txt");
                let item = getCacheItem(tem);
                props.reloadBookList(() => {
                    props.goChapterByName(s, item, () => {
                        props.setState({
                            loadingBook: false
                        })
                        DeleteEpubFile(tem).then(res => {
                            // hasError(tem)
                            console.log(res)
                        })
                        props.beginRecordTop(tem)

                        props.calculateFontLines()
                    });
                })

            })
        } else {
            let item = getCacheItem(tem);
            console.log('item', item)
            console.log('tem', tem)
            props.goChapterByName(tem, item, () => {
                props.beginRecordTop(tem)

                props.setState({
                    loadingBook: false

                })
                props.calculateFontLines()
            });
        }
    }

    useImperativeHandle(ref, () => ({
        selectBook: (tem) => {
            if (tem) {
                selectBookFun(tem)
            }
        }
    }))

    const books = sortBookList().filter(Boolean);
    const recentBooks = (getCacheItem("LastClickBook") || "").split(",").filter(Boolean);

    return isEmpty(state.currentBookChapterName) && props.display && isEmpty(state.errorInfo) ? (
        <section className="book-list" aria-label={t("我的书架")}>
            <div className="library-heading">
                <div className="library-heading-row">
                    <h1>{t("我的书架")}<span className="library-count">{books.length}</span></h1>
                    <span className="library-version">v{state.version || 'dev'}</span>
                    <div className="library-actions">
                        <Button type="primary" size="small" icon={<PlusOutlined/>}
                                aria-label={t("添加书籍")} title={t("添加书籍")} onClick={props.clickBookPlus}>
                            <span className="library-add-label">{t("添加书籍")}</span>
                        </Button>
                        <Button className="library-download-button" size="small" icon={<DownloadOutlined/>} aria-label={t("图书下载")}
                                title={t("图书下载")} onClick={() => props.onSelect({id: 'download'})}>
                            <span className="library-download-label">{t("图书下载")}</span>
                        </Button>
                    </div>
                </div>
                <div className="library-toolbar">
                    <Input.Search className="library-search" allowClear placeholder={t("搜索书名或拼音首字母")}
                                  aria-label={t("搜索书籍")} onSearch={props.onSearch}/>
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
                    const title = name.replace(/\.(txt|epub)$/i, '');
                    const isRecent = recentBooks[0] === name;
                    return <li key={name} className={classNames('library-book', {'library-book--recent': isRecent})}>
                        <button className="library-book-open" title={t("双击阅读：") + title}
                                onDoubleClick={() => selectBookFun(name)}
                                onKeyDown={event => {if (event.key === 'Enter') selectBookFun(name)}}>
                            <span className={'book-spine book-spine--' + index % 4} aria-hidden="true">
                                <span>{title.replace(/[《【「]/g, '').slice(0, 1)}</span>
                                <BookOutlined/>
                            </span>
                            <span className="library-book-info">
                                <span className="library-book-title">{title}</span>
                                <span className="library-book-meta">
                                    <span>{name.split('.').pop().toUpperCase()}</span>
                                    <span className="meta-dot">·</span>
                                    <span>{isRecent ? t("上次读到这里") : t("双击开始阅读")}</span>
                                </span>
                            </span>
                            {isRecent && <span className="reading-badge">{t("在读")}</span>}
                        </button>
                        <button className="book-delete" aria-label={t("删除 ") + title} title={t("删除书籍")}
                                onClick={() => DeleteFile(name).then(res => {
                                    if (!props.hasError(res)) {
                                        message.info(t("删除成功！"));
                                        props.reloadBookList();
                                    }
                                })}><DeleteOutlined/></button>
                    </li>
                })}
                {!books.length && <li className="library-empty">
                    <BookOutlined/>
                    <h2>{t("留一处安静的书架")}</h2>
                    <p>{t("添加一本书，或将文件拖入窗口。")}</p>
                    <Button onClick={props.clickBookPlus}>{t("添加第一本书")}</Button>
                </li>}
            </ul>
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
