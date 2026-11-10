import { Collapse, Form, Input, InputNumber, Select, Switch } from 'antd';
import { t } from '../i18n/index.jsx';

// 配置读取失败时也允许用户打开表单修复，非对象规则按空规则显示。
function asRules(value) {
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}

// 控件直接编辑 crawler 对象，保存时沿用现有字段名和 JSON 结构。
export default function CrawlerRuleFields({ value, hasSearch, onChange }) {
    const rules = asRules(value);

    // 更新一个字段时保留同级和其他分区的原配置，包括表单尚未支持的属性。
    function update(section, key, value) {
        if (section) {
            onChange({ ...rules, [section]: { ...asRules(rules[section]), [key]: value } });
        } else {
            onChange({ ...rules, [key]: value });
        }
    }

    function text(section, key, label, { required, placeholder, full } = {}) {
        const id = `crawler-${section}-${key}`;
        const value = asRules(rules[section])[key];
        return (
            <Form.Item
                label={t(label)}
                htmlFor={id}
                required={required}
                className={full ? 'resource-source-full' : undefined}
            >
                <Input
                    id={id}
                    value={typeof value === 'string' ? value : ''}
                    placeholder={placeholder}
                    spellCheck={false}
                    onChange={(event) => update(section, key, event.target.value)}
                />
            </Form.Item>
        );
    }

    function toggle(section, key, label) {
        const id = `crawler-${section}-${key}`;
        return (
            <Form.Item label={t(label)} htmlFor={id}>
                <Switch
                    id={id}
                    checked={asRules(rules[section])[key] === true}
                    onChange={(checked) => update(section, key, checked)}
                />
            </Form.Item>
        );
    }

    function number(key, label, max) {
        return (
            <Form.Item label={t(label)} htmlFor={`crawler-${key}`}>
                <InputNumber
                    id={`crawler-${key}`}
                    min={0}
                    max={max}
                    precision={0}
                    value={typeof rules[key] === 'number' ? rules[key] : null}
                    placeholder="0"
                    onChange={(value) => update(null, key, value ?? 0)}
                />
            </Form.Item>
        );
    }

    function panel(key, label, fields) {
        return { key, label: t(label), children: <div className="resource-source-fields">{fields}</div> };
    }

    const scriptNext = asRules(rules.chapter).scriptNext;
    const panels = [
        panel(
            'search',
            '搜索结果页',
            <>
                {text('search', 'result', '搜索结果条目选择器', { required: hasSearch, placeholder: '.result' })}
                {text('search', 'bookName', '书名链接选择器', { required: hasSearch, placeholder: '.title a' })}
                {text('search', 'author', '作者选择器')}
                {text('search', 'next', '搜索下一页选择器')}
                {text('search', 'cookies', '搜索 Cookie 请求头', { full: true })}
                {toggle('search', 'jsonp', '启用 JSONP 搜索')}
                {toggle('search', 'titlePrefix', '去掉书名前的方括号标签')}
            </>,
        ),
        panel(
            'book',
            '书籍详情页',
            <>
                {text('book', 'bookName', '书名选择器')}
                {text('book', 'author', '作者选择器')}
                {text('book', 'description', '简介选择器')}
                {text('book', 'category', '分类选择器')}
                {text('book', 'cover', '封面选择器')}
            </>,
        ),
        panel(
            'toc',
            '章节目录',
            <>
                {text('toc', 'item', '章节链接选择器', { required: true, placeholder: '#chapters a' })}
                {text('toc', 'nextPage', '目录分页选择器')}
                {text('toc', 'url', '目录页地址模板', { placeholder: '/book/{id}/index.html' })}
                {text('toc', 'baseUri', '章节链接基准地址')}
                {text('toc', 'hiddenList', '隐藏列表选择器')}
                {text('toc', 'onclickPath', '脚本章节链接模板', { placeholder: '/book/{id}/{chapter}.html' })}
                {toggle('toc', 'isDesc', '目录章节倒序排列')}
            </>,
        ),
        panel(
            'chapter',
            '章节正文',
            <>
                {text('chapter', 'content', '正文容器选择器', { required: true, placeholder: '#content' })}
                {text('chapter', 'title', '章节标题选择器')}
                {text('chapter', 'filterTag', '要移除的节点选择器')}
                {text('chapter', 'filterTxt', '要移除的文字正则')}
                {text('chapter', 'nextPage', '章内下一页选择器')}
                <Form.Item label={t('脚本翻页方式')} htmlFor="crawler-chapter-scriptNext">
                    <Select
                        id="crawler-chapter-scriptNext"
                        value={typeof scriptNext === 'string' && scriptNext ? scriptNext : undefined}
                        options={[{ value: 'nextpage' }, { value: 'keyboard' }]}
                        allowClear
                        onChange={(value) => update('chapter', 'scriptNext', value || '')}
                    />
                </Form.Item>
                {toggle('chapter', 'base64', '正文使用 Base64 编码')}
            </>,
        ),
    ];

    return (
        <>
            <div className="resource-source-fields">
                {number('minInterval', '请求间隔（毫秒）', 10000)}
                {number('concurrency', '协程数量', 5)}
            </div>
            <p className="resource-crawler-help">
                {t('选择器使用 CSS 语法。填写搜索地址时，还需配置搜索结果条目和书名链接。')}
            </p>
            <Collapse
                size="small"
                defaultActiveKey={hasSearch ? ['search', 'toc', 'chapter'] : ['toc', 'chapter']}
                items={panels}
            />
        </>
    );
}
