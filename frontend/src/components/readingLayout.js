import {DEFAULT_READING_TYPOGRAPHY} from './readingTypography.js';

export function calculateReadingLayout(height, fontSize, lineHeight, requestedRows = 0) {
    if (!Number.isFinite(height) || height < 1) return null;
    height = Math.floor(height);
    fontSize = Math.max(1, Number(fontSize) || DEFAULT_READING_TYPOGRAPHY.fontSize);
    lineHeight = Math.max(fontSize + 4, Number(lineHeight) || DEFAULT_READING_TYPOGRAPHY.lineHeight);
    const rows = Math.min(height, Math.max(1, requestedRows > 0
        ? Math.floor(requestedRows) : Math.floor(height / lineHeight)));
    const fittedLineHeight = Math.floor(height / rows);
    return {
        fontSize: Math.min(fontSize, Math.max(1, fittedLineHeight - 4)),
        fontLineHeight: fittedLineHeight,
        newContentHeight: fittedLineHeight * rows,
        blankLineHeight: 0,
    };
}
