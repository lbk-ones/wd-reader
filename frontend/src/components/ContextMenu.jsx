import React, {useState, useEffect, useLayoutEffect, useRef} from 'react';
import "./ContextMenu.css"

const ContextMenu = ({options, onSelect, onOpenWindow, fontSize = 14}) => {
    const [showMenu, setShowMenu] = useState(false);
    const [menuPosition, setMenuPosition] = useState({x: 0, y: 0});
    const menuRef = useRef(null);

    useLayoutEffect(() => {
        if (!showMenu) return;
        const {width, height} = menuRef.current.getBoundingClientRect();
        const x = Math.max(8, Math.min(menuPosition.x, window.innerWidth - width - 8));
        const y = Math.max(8, Math.min(menuPosition.y, window.innerHeight - height - 8));
        if (x !== menuPosition.x || y !== menuPosition.y) setMenuPosition({x, y});
    }, [showMenu, menuPosition.x, menuPosition.y, options]);

    useEffect(() => {
        const handleContextMenu = (e) => {
            e.preventDefault();
            if (onOpenWindow) {
                e.stopPropagation();
                setShowMenu(false);
                onOpenWindow(Math.round(e.clientX), Math.round(e.clientY));
                return;
            }
            setShowMenu(true);
            setMenuPosition({x: e.clientX, y: e.clientY});
        };

        const handleClick = (e) => {
            e.preventDefault();
            e.stopPropagation();
            setShowMenu(false);
            // console.log('点击。。。。')
        };
        let parentNode = menuRef.current.parentNode ? menuRef.current.parentNode : document;
        parentNode.addEventListener('contextmenu', handleContextMenu);
        document.addEventListener('click', handleClick);

        return () => {
            parentNode.removeEventListener('contextmenu', handleContextMenu);
            parentNode.removeEventListener('click', handleClick);
        };
    }, [onOpenWindow]);

    return (
        <div
            ref={menuRef}
            style={{
                display: showMenu ? 'block' : 'none',
                position: 'fixed',
                left: `${menuPosition.x}px`,
                top: `${menuPosition.y}px`,
                backgroundColor: 'white',
                border: '1px solid #ccc',
                borderRadius: '4px',
                boxShadow: '0 2px 4px rgba(0, 0, 0, 0.2)',
                zIndex: 1000,
                width: 'max-content',
                minWidth: '120px',
                maxWidth: 'calc(100vw - 16px)',
                lineHeight: '25px' // 写死吧 不然会受外面的影响
            }}
        >
            {options.map(option => (
                <div
                    key={option.id}
                    title={option.label}
                    className={'context-optional'}
                    style={{
                        cursor: 'pointer',
                        color: '#000',
                        width: '100%',
                        height: '100%',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        textAlign: 'center',
                        textIndent: 0,
                        ...(function () {

                            if (fontSize) {
                                return {
                                    fontSize
                                }
                            }
                            return {}
                        })()
                    }}
                    onClick={(e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        onSelect(option);
                        setShowMenu(false);
                    }}
                >
                    {option.label}
                </div>
            ))}
        </div>
    );
};

export default ContextMenu;
