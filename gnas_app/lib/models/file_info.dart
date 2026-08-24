class FileInfo {
  final String name;
  final String path;
  final bool isDir;
  final int size;
  final String modTime;

  FileInfo({
    required this.name,
    required this.path,
    required this.isDir,
    required this.size,
    required this.modTime,
  });

  factory FileInfo.fromJson(Map<String, dynamic> json) {
    return FileInfo(
      name: json['name'] as String? ?? '',
      path: json['path'] as String? ?? '',
      isDir: json['isDir'] as bool? ?? false,
      size: json['size'] as int? ?? 0,
      modTime: json['modTime'] as String? ?? '',
    );
  }

  String get formattedSize {
    if (isDir) return '';
    if (size < 1024) return '$size B';
    if (size < 1024 * 1024) return '${(size / 1024).toStringAsFixed(1)} KB';
    if (size < 1024 * 1024 * 1024) {
      return '${(size / (1024 * 1024)).toStringAsFixed(1)} MB';
    }
    return '${(size / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }

  bool get isImage {
    final ext = name.split('.').last.toLowerCase();
    return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].contains(ext);
  }

  bool get isVideo {
    final ext = name.split('.').last.toLowerCase();
    return ['mp4', 'webm', 'ogv', 'mov'].contains(ext);
  }
}