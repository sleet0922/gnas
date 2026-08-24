class MediaItem {
  final String name;
  final String path;
  final String type; // 'image' or 'video'
  final int size;
  final String modTime;

  MediaItem({
    required this.name,
    required this.path,
    required this.type,
    required this.size,
    required this.modTime,
  });

  factory MediaItem.fromJson(Map<String, dynamic> json) {
    return MediaItem(
      name: json['name'] as String? ?? '',
      path: json['path'] as String? ?? '',
      type: json['type'] as String? ?? 'image',
      size: json['size'] as int? ?? 0,
      modTime: json['modTime'] as String? ?? '',
    );
  }

  bool get isImage => type == 'image';
  bool get isVideo => type == 'video';
}

class DuplicateGroup {
  final double similarity;
  final List<MediaItem> items;

  DuplicateGroup({
    required this.similarity,
    required this.items,
  });

  factory DuplicateGroup.fromJson(Map<String, dynamic> json) {
    final rawItems = json['items'] as List<dynamic>? ?? [];
    return DuplicateGroup(
      similarity: (json['similarity'] as num?)?.toDouble() ?? 0.0,
      items: rawItems
          .map((e) => MediaItem.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }
}