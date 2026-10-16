export function createAutoPageTimer(tick, {schedule = setInterval, cancel = clearInterval} = {}) {
    let timer = null;
    let delay = 0;
    const stop = () => {
        if (timer !== null) cancel(timer);
        timer = null;
    };
    const start = interval => {
        stop();
        delay = interval;
        timer = schedule(tick, delay);
    };
    return {start, stop, reset: () => { if (timer !== null) start(delay); }};
}
