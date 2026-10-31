export function createAutoScroll({getContent, canScroll, onEnd,
    request = requestAnimationFrame, cancel = cancelAnimationFrame, now = () => performance.now()}) {
    let frame = null, active = false, speed = 20, endPause = 4;
    let movingContent = null;
    let previous = null, position = null, applied = null, bottomSince = null, pausedUntil = 0;
    const resetMotion = () => {
        movingContent?.style?.removeProperty?.('--reader-scroll-offset');
        movingContent = null;
        previous = position = applied = bottomSince = null;
    };
    const stop = () => {
        active = false;
        if (frame !== null) cancel(frame);
        frame = null;
        resetMotion();
    };
    const step = timestamp => {
        frame = null;
        if (!active) return;
        const content = getContent();
        if (!content || content.clientHeight <= 0 || !canScroll() || timestamp < pausedUntil) {
            resetMotion();
        } else {
            const limit = Math.max(0, content.scrollHeight - content.clientHeight);
            if (previous === null || (applied !== null && Math.abs(content.scrollTop - applied) > 1)) {
                position = content.scrollTop;
                previous = timestamp;
            }
            const elapsed = Math.max(0, Math.min(100, timestamp - previous));
            previous = timestamp;
            position = Math.min(limit, position + speed * elapsed / 1000);
            content.scrollTop = position;
            applied = content.scrollTop;
            movingContent = content;
            content.style?.setProperty?.('--reader-scroll-offset', `${position < limit ? applied - position : 0}px`);
            if (limit - content.scrollTop <= 0.5) {
                if (bottomSince === null) bottomSince = timestamp;
                // Leave time to finish the last stationary screen before advancing.
                const dwell = Math.max(endPause * 1000, content.clientHeight / speed * 1000);
                if (timestamp - bottomSince >= dwell) {
                    stop();
                    onEnd();
                    return;
                }
            } else {
                bottomSince = null;
            }
        }
        if (active) frame = request(step);
    };
    return {
        start(value, pauseSeconds = 4) {
            stop();
            speed = Math.min(100, Math.max(5, Number(value) || 20));
            endPause = Math.min(120, Math.max(1, Number(pauseSeconds) || 4));
            pausedUntil = 0;
            active = true;
            frame = request(step);
        },
        stop,
        pause(milliseconds = 1500) {
            pausedUntil = now() + milliseconds;
            resetMotion();
        },
    };
}
