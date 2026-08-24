class ApiResponse<T> {
  final int code;
  final String? message;
  final T? data;

  ApiResponse({required this.code, this.message, this.data});

  bool get isSuccess => code == 0;

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic)? dataParser,
  ) {
    return ApiResponse(
      code: json['code'] as int? ?? 1,
      message: json['message'] as String?,
      data: json['data'] != null && dataParser != null
          ? dataParser(json['data'])
          : json['data'] as T?,
    );
  }
}