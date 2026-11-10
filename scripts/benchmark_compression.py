"""Compare lossless compression on one file with Python 3.14 or later.

Install dependencies with `python -m pip install Brotli lz4`.
Run `python scripts/benchmark_compression.py FILE --output-dir NEW_DIRECTORY`.
"""

import argparse
import bz2
import csv
import gzip
import hashlib
import lzma
import platform
from collections.abc import Callable
from compression import zstd
from dataclasses import asdict, dataclass, fields
from importlib.metadata import version
from pathlib import Path
from statistics import median
from time import perf_counter_ns

import brotli
import lz4.frame


@dataclass(frozen=True)
class Codec:
    name: str
    extension: str
    levels: tuple[int, ...]
    compress: Callable[[bytes, int], bytes]
    decompress: Callable[[bytes], bytes]


CODECS = (
    Codec("LZ4", "lz4", (0, 12),
          lambda data, level: lz4.frame.compress(data, compression_level=level),
          lz4.frame.decompress),
    Codec("Zstandard", "zst", (1, 3, 9, 19, 22), zstd.compress, zstd.decompress),
    Codec("gzip", "gz", (1, 6, 9),
          lambda data, level: gzip.compress(data, compresslevel=level, mtime=0),
          gzip.decompress),
    Codec("bzip2", "bz2", (1, 9), bz2.compress, bz2.decompress),
    Codec("XZ", "xz", (0, 6, 9),
          lambda data, level: lzma.compress(data, preset=level), lzma.decompress),
    Codec("Brotli", "br", (1, 5, 9, 11),
          lambda data, level: brotli.compress(data, quality=level), brotli.decompress),
)


@dataclass(frozen=True)
class Result:
    algorithm: str
    level: int
    original_bytes: int
    compressed_bytes: int
    savings_percent: float
    compress_ms: float
    decompress_ms: float


def benchmark(data: bytes, codec: Codec, level: int, runs: int) -> tuple[Result, bytes]:
    compress_times = []
    decompress_times = []
    # Warm up and verify once before collecting the timed samples.
    for run in range(runs + 1):
        start = perf_counter_ns()
        packed = codec.compress(data, level)
        compressed_at = perf_counter_ns()
        restored = codec.decompress(packed)
        decompressed_at = perf_counter_ns()
        if restored != data:
            raise ValueError(f"Round trip failed for {codec.name} level {level}")
        if run:
            compress_times.append(compressed_at - start)
            decompress_times.append(decompressed_at - compressed_at)
        del restored
        if run < runs:
            del packed

    result = Result(
        codec.name, level, len(data), len(packed),
        100 * (len(data) - len(packed)) / len(data),
        median(compress_times) / 1_000_000,
        median(decompress_times) / 1_000_000,
    )
    return result, packed


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("file", type=Path)
    parser.add_argument("--runs", type=int, default=5, help="timed runs per level (default: 5)")
    parser.add_argument("--output-dir", type=Path, help="new directory for the CSV and two winners")
    args = parser.parse_args()
    if args.runs < 1:
        parser.error("--runs must be positive")
    if args.output_dir is not None and args.output_dir.exists():
        parser.error("--output-dir must be a new directory")
    data = args.file.read_bytes()
    if not data:
        parser.error("the input file must be nonempty")

    print(f"Python {platform.python_version()}, Zstd {zstd.zstd_version}, "
          f"Brotli {version('Brotli')}, lz4 {version('lz4')}")
    print(f"{len(data):,} bytes, SHA-256 {hashlib.sha256(data).hexdigest()}")
    print(f"Median of {args.runs} timed runs, one warmup, in memory", flush=True)
    print(f"{'Algorithm':<10} {'Level':>5} {'Bytes':>11} {'Saved':>8} "
          f"{'Comp ms':>10} {'Decomp ms':>10}")

    results = []
    smallest = fastest = None
    for codec in CODECS:
        for level in codec.levels:
            result, packed = benchmark(data, codec, level, args.runs)
            results.append(result)
            print(f"{codec.name:<10} {level:>5} {result.compressed_bytes:>11,} "
                  f"{result.savings_percent:>7.2f}% {result.compress_ms:>10.2f} "
                  f"{result.decompress_ms:>10.2f}", flush=True)
            candidate = (result, codec.extension, packed)
            if smallest is None or result.compressed_bytes < smallest[0].compressed_bytes:
                smallest = candidate
            if fastest is None or result.compress_ms < fastest[0].compress_ms:
                fastest = candidate

    if args.output_dir is not None:
        args.output_dir.mkdir()
        with (args.output_dir / "results.csv").open("w", encoding="utf-8", newline="") as file:
            writer = csv.DictWriter(file, fieldnames=[field.name for field in fields(Result)])
            writer.writeheader()
            writer.writerows(asdict(result) for result in results)
        for label, (result, extension, packed) in (("smallest", smallest), ("fastest", fastest)):
            name = f"{label}-{result.algorithm.lower()}-{result.level}.{extension}"
            (args.output_dir / name).write_bytes(packed)


if __name__ == "__main__":
    main()
