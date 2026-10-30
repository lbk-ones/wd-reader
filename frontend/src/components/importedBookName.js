// The importer keeps TXT files and converts the other supported formats to TXT.
export function importedBookName(sourcePath) {
    const name = sourcePath.split(/[\\/]/).pop() || '';
    if (/\.txt$/i.test(name)) return name;
    return name.replace(/\.[^.]+$/, '.txt');
}
