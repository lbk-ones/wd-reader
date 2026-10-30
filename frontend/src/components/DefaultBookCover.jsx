import {useLayoutEffect, useRef} from 'react';

export default function DefaultBookCover({title}) {
    const boxRef = useRef(null);
    const titleRef = useRef(null);

    useLayoutEffect(() => {
        const box = boxRef.current;
        const text = titleRef.current;
        if (!box || !text) return;
        let active = true;
        const fit = () => {
            if (!active || !box.clientWidth || !box.clientHeight) return;
            let low = 1, high = Math.min(20, box.clientWidth / 2), best = low;
            for (let i = 0; i < 10; i++) {
                const size = (low + high) / 2;
                text.style.fontSize = `${size}px`;
                if (text.scrollHeight <= box.clientHeight && text.scrollWidth <= box.clientWidth) {
                    best = size;
                    low = size;
                } else {
                    high = size;
                }
            }
            text.style.fontSize = `${best}px`;
        };
        fit();
        const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(fit);
        observer?.observe(box);
        document.fonts?.ready.then(fit);
        return () => { active = false; observer?.disconnect(); };
    }, [title]);

    return <span ref={boxRef} className="default-book-cover">
        <span ref={titleRef} className="default-book-cover-title">{title}</span>
    </span>;
}
