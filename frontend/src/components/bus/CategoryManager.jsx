import {useState} from 'react';
import {Button, Drawer, Input, Modal, message} from 'antd';
import {t, useI18n} from '../../i18n/index.jsx';
import {SaveCategory, DeleteCategory} from '../../../bindings/wd-reader/internal/desktop/app.js';

export default function CategoryManager({open, onClose, categories, onChanged}) {
    useI18n();
    const [editingID, setEditingID] = useState('');
    const [name, setName] = useState('');
    const [busy, setBusy] = useState(false);

    async function save() {
        if (busy || !name.trim()) return;
        setBusy(true);
        try {
            await SaveCategory(editingID, name.trim());
            setEditingID('');
            setName('');
            await onChanged();
        } catch (error) {
            message.error(String(error));
        } finally {
            setBusy(false);
        }
    }

    async function remove(id) {
        setBusy(true);
        try {
            await DeleteCategory(id);
            if (editingID === id) {
                setEditingID('');
                setName('');
            }
            await onChanged();
        } catch (error) {
            message.error(String(error));
        } finally {
            setBusy(false);
        }
    }

    return <Drawer className="category-manager" title={t('管理分类')} open={open} onClose={onClose} zIndex={1100}
                   width="min(420px, 100vw)">
        <p className="original-library-description">{t('在编辑书籍信息中为书籍选择分类。')}</p>
        <div className="category-editor">
            <Input aria-label={t('分类名称')} placeholder={t('分类名称')} maxLength={64} value={name}
                   disabled={busy} onChange={event => setName(event.target.value)} onPressEnter={save}/>
            <Button type="primary" loading={busy} disabled={!name.trim()} onClick={save}>
                {t(editingID ? '保存' : '新增分类')}
            </Button>
            {editingID && <Button disabled={busy} onClick={() => {setEditingID(''); setName('');}}>{t('取消')}</Button>}
        </div>
        <ul className="category-manager-list">
            {categories.map(category => <li key={category.id}>
                <span title={category.name}>{category.name}</span>
                <Button size="small" disabled={busy} onClick={() => {setEditingID(category.id); setName(category.name);}}>{t('重命名')}</Button>
                <Button size="small" danger disabled={busy} onClick={() => Modal.confirm({
                    zIndex: 1200,
                    title: t('删除分类'),
                    content: t('删除分类后书籍保留，并取消分类关联。'),
                    okText: t('删除分类'),
                    cancelText: t('取消'),
                    onOk: () => remove(category.id),
                })}>{t('删除分类')}</Button>
            </li>)}
        </ul>
    </Drawer>;
}
