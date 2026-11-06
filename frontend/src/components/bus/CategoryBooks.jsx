import { useState } from 'react';
import { Button, Checkbox, Drawer, Input, message } from 'antd';
import { t, useI18n } from '../../i18n/index.jsx';
import { AssignUncategorizedBooks } from '../../../bindings/wd-reader/internal/desktop/app.js';

export default function CategoryBooks({ category, books, onClose, onSaved }) {
    useI18n();
    const [query, setQuery] = useState('');
    const [selected, setSelected] = useState([]);
    const [saving, setSaving] = useState(false);
    const needle = query.trim().toLowerCase();
    const visible = books.filter((book) =>
        [book.title, book.fileName, ...(book.authors || [])].some((value) => value?.toLowerCase().includes(needle)),
    );

    async function save() {
        if (saving || !selected.length) return;
        setSaving(true);
        try {
            await AssignUncategorizedBooks(category.id, selected);
            await onSaved();
            onClose();
        } catch (error) {
            message.error(String(error));
        } finally {
            setSaving(false);
        }
    }

    return (
        <Drawer
            className="category-books"
            title={t('维护分类：{0}', { 0: category.name })}
            open
            width="min(420px, 100vw)"
            onClose={() => {
                if (!saving) onClose();
            }}
            footer={
                <Button type="primary" block loading={saving} disabled={!selected.length} onClick={save}>
                    {t('加入分类（{0}）', { 0: selected.length })}
                </Button>
            }
        >
            <p className="original-library-description">{t('选择尚未分类的书籍，加入当前分类。')}</p>
            <Input.Search
                aria-label={t('搜索书籍')}
                placeholder={t('搜索书名或作者')}
                value={query}
                allowClear
                onChange={(event) => setQuery(event.target.value)}
            />
            <div className="category-books-list">
                {visible.map((book) => (
                    <Checkbox
                        key={book.id}
                        checked={selected.includes(book.id)}
                        disabled={saving}
                        onChange={(event) =>
                            setSelected((current) =>
                                event.target.checked ? [...current, book.id] : current.filter((id) => id !== book.id),
                            )
                        }
                    >
                        <span className="category-books-title">{book.title || book.fileName}</span>
                        {!!book.authors?.length && <small>{book.authors.join('、')}</small>}
                    </Checkbox>
                ))}
            </div>
            {!visible.length && (
                <p className="library-empty">{t(books.length ? '没有找到匹配的书籍' : '没有尚未分类的书籍')}</p>
            )}
        </Drawer>
    );
}
