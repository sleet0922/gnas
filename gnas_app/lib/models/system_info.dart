class SystemInfo {
  final AIStatus ai;
  final String os;
  final String arch;
  final int cpuCores;
  final double cpuUsage;
  final double procCPU;
  final int memoryTotal;
  final int memoryUsed;
  final int memoryFree;
  final int procMem;
  final int procMemSys;
  final int diskTotal;
  final int diskUsed;
  final int diskFree;
  final double uptime;
  final int dbSize;
  final String dbSizeString;

  SystemInfo({
    required this.ai,
    required this.os,
    required this.arch,
    required this.cpuCores,
    required this.cpuUsage,
    required this.procCPU,
    required this.memoryTotal,
    required this.memoryUsed,
    required this.memoryFree,
    required this.procMem,
    required this.procMemSys,
    required this.diskTotal,
    required this.diskUsed,
    required this.diskFree,
    required this.uptime,
    required this.dbSize,
    required this.dbSizeString,
  });

  factory SystemInfo.fromJson(Map<String, dynamic> json) {
    return SystemInfo(
      ai: AIStatus.fromJson(json['ai'] as Map<String, dynamic>? ?? const {}),
      os: json['os'] as String? ?? '',
      arch: json['arch'] as String? ?? '',
      cpuCores: json['cpuCores'] as int? ?? 0,
      cpuUsage: (json['cpuUsage'] as num?)?.toDouble() ?? 0,
      procCPU: (json['procCPU'] as num?)?.toDouble() ?? 0,
      memoryTotal: json['memoryTotal'] as int? ?? 0,
      memoryUsed: json['memoryUsed'] as int? ?? 0,
      memoryFree: json['memoryFree'] as int? ?? 0,
      procMem: json['procMem'] as int? ?? 0,
      procMemSys: json['procMemSys'] as int? ?? 0,
      diskTotal: json['diskTotal'] as int? ?? 0,
      diskUsed: json['diskUsed'] as int? ?? 0,
      diskFree: json['diskFree'] as int? ?? 0,
      uptime: (json['uptime'] as num?)?.toDouble() ?? 0,
      dbSize: json['dbSize'] as int? ?? 0,
      dbSizeString: json['dbSizeString'] as String? ?? '',
    );
  }

  double get memoryUsagePercent =>
      memoryTotal > 0 ? (memoryUsed / memoryTotal) * 100 : 0;
  double get diskUsagePercent =>
      diskTotal > 0 ? (diskUsed / diskTotal) * 100 : 0;

  String get formattedUptime {
    final days = (uptime / 86400).floor();
    final hours = ((uptime % 86400) / 3600).floor();
    final minutes = ((uptime % 3600) / 60).floor();
    if (days > 0) return '${days}d ${hours}h ${minutes}m';
    if (hours > 0) return '${hours}h ${minutes}m';
    return '${minutes}m';
  }
}

class SystemStatus {
  final String version;
  final String username;

  SystemStatus({required this.version, required this.username});

  factory SystemStatus.fromJson(Map<String, dynamic> json) {
    return SystemStatus(
      version: json['version'] as String? ?? '',
      username: json['username'] as String? ?? '',
    );
  }
}

class AIStatus {
  final bool enabled;
  final AIServiceStatus model;
  final AIServiceStatus qdrant;

  const AIStatus({
    required this.enabled,
    required this.model,
    required this.qdrant,
  });

  factory AIStatus.fromJson(Map<String, dynamic> json) {
    return AIStatus(
      enabled: json['enabled'] as bool? ?? false,
      model: AIServiceStatus.fromJson(
        json['model'] as Map<String, dynamic>? ?? const {},
      ),
      qdrant: AIServiceStatus.fromJson(
        json['qdrant'] as Map<String, dynamic>? ?? const {},
      ),
    );
  }
}

class AIServiceStatus {
  final String status;
  final String message;
  final String? version;
  final String? device;

  const AIServiceStatus({
    required this.status,
    required this.message,
    this.version,
    this.device,
  });

  factory AIServiceStatus.fromJson(Map<String, dynamic> json) {
    return AIServiceStatus(
      status: json['status'] as String? ?? 'unavailable',
      message: json['message'] as String? ?? '',
      version: json['version'] as String?,
      device: json['device'] as String?,
    );
  }
}
