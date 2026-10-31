export function calculatePageTurn({scrollTop, viewportHeight, contentHeight, lineHeight, origin = 0, direction}) {
    if (![scrollTop, viewportHeight, contentHeight, lineHeight, origin].every(Number.isFinite)
        || lineHeight <= 0 || viewportHeight < lineHeight) return null;
    const pageHeight = Math.floor(viewportHeight / lineHeight) * lineHeight;
    const overflow = Math.max(0, contentHeight - viewportHeight);
    // Allow the last page to reach a line boundary instead of having the
    // browser clamp an aligned target back to a fractional line at chapter end.
    const maxTop = overflow > 0
        ? Math.max(0, origin + Math.ceil((overflow - origin) / lineHeight - 1e-7) * lineHeight) : 0;
    const desired = (scrollTop + (direction > 0 ? pageHeight : -pageHeight) - origin) / lineHeight;
    // Overlap the partially visible edge line rather than skipping unread text.
    const row = direction > 0 ? Math.floor(desired + 1e-7) : Math.ceil(desired - 1e-7);
    return {
        top: Math.max(0, Math.min(maxTop, origin + row * lineHeight)),
        pageHeight,
        paddingBottom: Math.max(0, maxTop - overflow),
    };
}

export function turnReaderPage(content, lineHeight, direction) {
    if (!content || content.clientHeight <= 0) return;
    const scrollTop = content.scrollTop;
    const firstParagraph = content.querySelector(':scope > .reader-text > p, :scope > p');
    const origin = firstParagraph
        ? firstParagraph.getBoundingClientRect().top - content.getBoundingClientRect().top - content.clientTop + scrollTop
        : 0;
    const oldPadding = parseFloat(content.style.paddingBottom) || 0;
    const turn = calculatePageTurn({scrollTop, viewportHeight: content.clientHeight,
        contentHeight: content.scrollHeight - oldPadding, lineHeight: Number(lineHeight), origin, direction});
    if (!turn) return;
    content.style.paddingBottom = `${turn.paddingBottom}px`;
    content.scrollTo({top: turn.top, behavior: 'instant'});
}
