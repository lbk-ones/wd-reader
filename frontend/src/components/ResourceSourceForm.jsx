import { Button, Form, Input, InputNumber, Select, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { SourceKind } from '../../bindings/wd-reader/internal/resourcecatalog/models.js';
import { t } from '../i18n/index.jsx';
import { defaultSourceGroup, normalizeSourceGroup } from './resourceDownloads.js';

// 类型键写入配置，显示名称由界面翻译。
export const sourceKindLabels = {
    [SourceKind.HTMLKind]: '网页书源',
    [SourceKind.CrawlerKind]: '网页小说抓取',
    [SourceKind.FileIndexKind]: '文件目录',
    [SourceKind.ArchiveKind]: 'Internet Archive',
    [SourceKind.GutenbergKind]: 'Project Gutenberg',
};
const formats = ['txt', 'epub', 'mobi', 'azw', 'azw3'];
const formatOptions = formats.map((value) => ({ value, label: value.toUpperCase() }));
const mappingFields = ['searchForm', 'authorHints', 'aliases'];
// 切换类型时用这些字段列表找出应移除的专用属性。分组、主页等通用字段独立保留。
const kindFields = {
    [SourceKind.HTMLKind]: [
        'searchUrl',
        'searchMethod',
        'maxSearchPages',
        'bookLinkPattern',
        'downloadLinkPattern',
        'downloadPagePattern',
        'downloadFormat',
        'zipFormat',
        'fileNameFromLink',
        'titlePattern',
        'authorPattern',
        'titleMeta',
        'authorMeta',
        ...mappingFields,
    ],
    [SourceKind.CrawlerKind]: [
        'searchUrl',
        'searchMethod',
        'searchForm',
        'maxSearchPages',
        'bookLinkPattern',
        'titleMeta',
        'authorMeta',
        'crawler',
    ],
    [SourceKind.FileIndexKind]: ['indexUrl', 'downloadUrl', 'bookUrl', 'authorHints'],
    [SourceKind.ArchiveKind]: ['searchUrl', 'detailUrl', 'downloadUrl', 'bookUrl'],
    [SourceKind.GutenbergKind]: ['searchUrl', 'detailUrl', 'bookUrl'],
};

// 把配置原文转换成表单草稿。映射转为可增删的行，抓取规则转为可直接编辑的 JSON 文本。
export function readConfigDraft(content) {
    const config = JSON.parse(content);
    if (
        !Array.isArray(config?.sources) ||
        config.sources.some((source) => !source || typeof source !== 'object' || Array.isArray(source))
    ) {
        throw new Error(t('无法读取书源列表'));
    }
    return {
        ...config,
        sources: config.sources.map((source) => {
            const draft = { ...source };
            if ('group' in source) draft.group = normalizeSourceGroup(source.group);
            for (const key of mappingFields) {
                if (key in source)
                    draft[key] = Object.entries(source[key] || {}).map(([name, value]) => ({ name, value }));
            }
            if (source.kind === SourceKind.CrawlerKind) {
                draft.crawler = JSON.stringify(source.crawler || {}, null, 2);
            }
            return draft;
        }),
    };
}

// 将草稿还原为后端配置格式。保存前检查映射键和抓取规则的 JSON 结构。
export function writeConfigDraft(config) {
    const sources = config.sources.map((source) => {
        const result = { ...source };
        if ('group' in source) result.group = normalizeSourceGroup(source.group);
        for (const key of mappingFields) {
            if (!(key in source)) continue;
            const names = new Set();
            const pairs = source[key].map(({ name, value }) => {
                const field = name.trim();
                if (!field || names.has(field)) {
                    throw new Error(t('来源“{0}”的映射名称为空或重复', { 0: source.name || source.id }));
                }
                // 先检查重复键，避免转成 JSON 对象时后一行覆盖前一行。
                names.add(field);
                return [field, value];
            });
            result[key] = Object.fromEntries(pairs);
        }
        if (source.kind === SourceKind.CrawlerKind) {
            try {
                const crawler = JSON.parse(source.crawler || '{}');
                if (!crawler || typeof crawler !== 'object' || Array.isArray(crawler)) throw new Error();
                result.crawler = crawler;
            } catch {
                throw new Error(t('来源“{0}”的网页抓取规则需要有效的 JSON 对象', { 0: source.name || source.id }));
            }
        }
        return result;
    });
    return JSON.stringify({ ...config, sources }, null, 2) + '\n';
}

// 渲染表单参数、作者提示或书名别名等键值列表。multiple 用于允许一键对应多个别名。
function MappingFields({ label, rows = [], nameLabel, valueLabel, multiple, onChange }) {
    // 用新行替换当前编辑项，方便父级比较和保存草稿。
    function update(index, field, value) {
        onChange(rows.map((row, i) => (i === index ? { ...row, [field]: value } : row)));
    }
    return (
        <div className="resource-mapping-fields">
            <div className="resource-mapping-heading">{t(label)}</div>
            {rows.map((row, index) => (
                <div className="resource-mapping-row" key={index}>
                    <Input
                        aria-label={t(nameLabel)}
                        placeholder={t(nameLabel)}
                        value={row.name}
                        onChange={(event) => update(index, 'name', event.target.value)}
                    />
                    {multiple ? (
                        <Select
                            mode="tags"
                            aria-label={t(valueLabel)}
                            placeholder={t('输入别名后按回车')}
                            value={Array.isArray(row.value) ? row.value : []}
                            onChange={(value) => update(index, 'value', value)}
                        />
                    ) : (
                        <Input
                            aria-label={t(valueLabel)}
                            placeholder={t(valueLabel)}
                            value={row.value}
                            onChange={(event) => update(index, 'value', event.target.value)}
                        />
                    )}
                    <Button
                        type="text"
                        icon={<DeleteOutlined />}
                        aria-label={t('移除此行')}
                        onClick={() => onChange(rows.filter((_, i) => i !== index))}
                    />
                </div>
            ))}
            <Button
                size="small"
                icon={<PlusOutlined />}
                onClick={() => onChange([...rows, { name: '', value: multiple ? [] : '' }])}
            >
                {t('添加一行')}
            </Button>
        </div>
    );
}

// 编辑一个书源草稿。通用字段共用表单控件，crawler 对象单独用 JSON 编辑。
export default function ResourceSourceForm({ source, onChange, disabled }) {
    const html = source.kind === SourceKind.HTMLKind;
    const crawler = source.kind === SourceKind.CrawlerKind;
    const index = source.kind === SourceKind.FileIndexKind;
    const post = String(source.searchMethod || '').toUpperCase() === 'POST';
    const change = (key, value) => onChange({ ...source, [key]: value });
    // 保留两个类型共同使用的字段，只清掉旧类型独有的规则。
    function changeKind(kind) {
        const next = { ...source, kind };
        for (const key of kindFields[source.kind] || []) {
            if (!kindFields[kind].includes(key)) delete next[key];
        }
        // 仅在分组仍是旧类型默认值时切换默认分组，用户填写的自定义组名继续沿用。
        if (kind === SourceKind.CrawlerKind && next.group === defaultSourceGroup) next.group = 'main';
        if (crawler && kind !== SourceKind.CrawlerKind && next.group === 'main') next.group = defaultSourceGroup;
        onChange(next);
    }
    // 创建绑定到同名草稿字段的单行或多行输入框。
    function text(key, label, { required, placeholder, full, multiline, rows } = {}) {
        const Component = multiline ? Input.TextArea : Input;
        return (
            <Form.Item
                label={t(label)}
                htmlFor={`source-${key}`}
                required={required}
                className={full ? 'resource-source-full' : undefined}
            >
                <Component
                    id={`source-${key}`}
                    value={source[key] ?? ''}
                    placeholder={placeholder}
                    spellCheck={false}
                    rows={rows}
                    onChange={(event) => change(key, event.target.value)}
                />
            </Form.Item>
        );
    }
    // 文件格式使用固定选项，清空时写入空字符串，交给对应的后端默认规则。
    function format(key, label) {
        return (
            <Form.Item label={t(label)} htmlFor={`source-${key}`}>
                <Select
                    id={`source-${key}`}
                    value={source[key] || undefined}
                    options={formatOptions}
                    allowClear
                    onChange={(value) => change(key, value || '')}
                />
            </Form.Item>
        );
    }
    return (
        <Form layout="vertical" disabled={disabled} className="resource-source-form" component="div">
            <h3>{t('基本信息')}</h3>
            <div className="resource-source-fields">
                {text('name', '来源名称', { required: true })}
                {text('id', '来源标识', { required: true })}
                <Form.Item label={t('来源类型')} htmlFor="source-kind" required>
                    <Select
                        id="source-kind"
                        value={source.kind}
                        onChange={changeKind}
                        options={Object.entries(sourceKindLabels).map(([value, label]) => ({ value, label: t(label) }))}
                    />
                </Form.Item>
                <Form.Item label={t('启用来源')} htmlFor="source-enabled">
                    <Switch
                        id="source-enabled"
                        checked={!!source.enabled}
                        onChange={(value) => change('enabled', value)}
                    />
                </Form.Item>
                {text('group', '书源分组')}
                {text('homeUrl', '书源主页', { required: crawler, placeholder: 'https://example.org/' })}
                {text('description', '来源说明', { full: true, multiline: true })}
            </div>
            <h3>{t('检索与下载')}</h3>
            <div className="resource-source-fields">
                {index
                    ? text('indexUrl', '文件目录地址', { required: true, full: true })
                    : text('searchUrl', '搜索地址', {
                          required: !crawler,
                          full: true,
                          placeholder: 'https://example.org/search?q={query}',
                      })}
                {!html &&
                    !crawler &&
                    !index &&
                    text('detailUrl', '详情接口地址', {
                        required: true,
                        full: true,
                        placeholder: 'https://example.org/api/{id}',
                    })}
                {!html &&
                    !crawler &&
                    text('bookUrl', '书籍详情地址', {
                        required: true,
                        full: true,
                        placeholder: index ? 'https://example.org/books/{file}' : 'https://example.org/books/{id}',
                    })}
                {(index || source.kind === SourceKind.ArchiveKind) &&
                    text('downloadUrl', '下载地址模板', {
                        required: true,
                        full: true,
                        placeholder: index
                            ? 'https://example.org/download/{file}'
                            : 'https://example.org/download/{id}/{file}',
                    })}
                {(html || crawler) && (
                    <>
                        <Form.Item label={t('搜索请求方式')} htmlFor="source-searchMethod">
                            <Select
                                id="source-searchMethod"
                                value={post ? 'POST' : 'GET'}
                                onChange={(value) => change('searchMethod', value)}
                                options={[{ value: 'GET' }, { value: 'POST' }]}
                            />
                        </Form.Item>
                        <Form.Item label={t('最多搜索页数')} htmlFor="source-maxSearchPages">
                            <InputNumber
                                id="source-maxSearchPages"
                                min={0}
                                max={10}
                                precision={0}
                                value={source.maxSearchPages ?? 0}
                                onChange={(value) => change('maxSearchPages', value ?? 0)}
                            />
                        </Form.Item>
                        {text('bookLinkPattern', '书籍链接正则', { required: html })}
                    </>
                )}
                {html && (
                    <>
                        {text('downloadLinkPattern', '下载链接正则', { required: true })}
                        {text('downloadPagePattern', '中转页链接正则', { full: true })}
                        {format('downloadFormat', '无扩展名文件的格式')}
                        {format('zipFormat', 'ZIP 内部文件格式')}
                        <Form.Item label={t('使用链接文字作为文件名')} htmlFor="source-fileNameFromLink">
                            <Switch
                                id="source-fileNameFromLink"
                                checked={!!source.fileNameFromLink}
                                onChange={(value) => change('fileNameFromLink', value)}
                            />
                        </Form.Item>
                    </>
                )}
            </div>
            {(html || crawler) && post && (
                <MappingFields
                    label="POST 表单参数"
                    nameLabel="参数名"
                    valueLabel="参数值"
                    rows={source.searchForm}
                    onChange={(value) => change('searchForm', value)}
                />
            )}
            {crawler && (
                <>
                    <h3>{t('逐章下载')}</h3>
                    <div className="resource-source-fields">
                        {text('titleMeta', '书名 Meta 字段')}
                        {text('authorMeta', '作者 Meta 字段')}
                        {text('crawler', '网页抓取规则（JSON）', {
                            required: true,
                            full: true,
                            multiline: true,
                            rows: 11,
                            placeholder: '{ "toc": { "item": "#chapters a" }, "chapter": { "content": "#content" } }',
                        })}
                    </div>
                </>
            )}
            {(html || index) && (
                <>
                    <h3>{t('书名与作者')}</h3>
                    {html && (
                        <div className="resource-source-fields">
                            {text('titlePattern', '书名提取正则')}
                            {text('authorPattern', '作者提取正则')}
                            {text('titleMeta', '书名 Meta 字段')}
                            {text('authorMeta', '作者 Meta 字段')}
                        </div>
                    )}
                    <MappingFields
                        label="作者提示"
                        nameLabel="书名"
                        valueLabel="作者"
                        rows={source.authorHints}
                        onChange={(value) => change('authorHints', value)}
                    />
                    {html && (
                        <MappingFields
                            label="书名别名"
                            nameLabel="书名"
                            valueLabel="别名"
                            multiple
                            rows={source.aliases}
                            onChange={(value) => change('aliases', value)}
                        />
                    )}
                </>
            )}
        </Form>
    );
}
