-- 创建数据库
CREATE DATABASE IF NOT EXISTS seas DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

-- 使用数据库
USE seas;

-- ============================================
-- 1. 班级表
-- ============================================
CREATE TABLE IF NOT EXISTS classes (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '班级ID',
  name VARCHAR(100) UNIQUE NOT NULL COMMENT '班级名称',
  grade VARCHAR(50) COMMENT '年级',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  INDEX idx_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='班级表';

-- ============================================
-- 2. 学生表
-- ============================================
CREATE TABLE IF NOT EXISTS students (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '学生ID',
  student_number VARCHAR(64) UNIQUE NOT NULL COMMENT '学号',
  name VARCHAR(100) NOT NULL COMMENT '学生姓名',
  class_id BIGINT NOT NULL COMMENT '班级ID',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  INDEX idx_student_number (student_number),
  INDEX idx_class_id (class_id),
  FOREIGN KEY (class_id) REFERENCES classes(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='学生表';

-- ============================================
-- 3. 学科表
-- ============================================
CREATE TABLE IF NOT EXISTS subjects (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '学科ID',
  name VARCHAR(100) NOT NULL COMMENT '学科名称',
  code VARCHAR(50) COMMENT '学科编码',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE INDEX idx_code (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='学科表';

-- ============================================
-- 4. 考试表
-- ============================================
CREATE TABLE IF NOT EXISTS exams (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '考试ID',
  name VARCHAR(100) NOT NULL COMMENT '考试名称',
  exam_date TIMESTAMP NOT NULL COMMENT '考试时间',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  INDEX idx_exam_date (exam_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='考试表';

-- ============================================
-- 5. 考试-学科关联表
-- ============================================
CREATE TABLE IF NOT EXISTS exam_subjects (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '关联ID',
  exam_id BIGINT NOT NULL COMMENT '考试ID',
  subject_id BIGINT NOT NULL COMMENT '学科ID',
  full_score FLOAT NOT NULL DEFAULT 100 COMMENT '该科满分',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE INDEX idx_exam_subject (exam_id, subject_id),
  FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE,
  FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='考试学科关联表';

-- ============================================
-- 6. 学生成绩表（汇总分数）
-- ============================================
CREATE TABLE IF NOT EXISTS scores (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '成绩ID',
  student_id BIGINT NOT NULL COMMENT '学生ID',
  exam_id BIGINT NOT NULL COMMENT '考试ID',
  subject_id BIGINT NOT NULL COMMENT '学科ID',
  total_score FLOAT NOT NULL COMMENT '该科总分',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  UNIQUE INDEX idx_student_exam_subject (student_id, exam_id, subject_id),
  FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
  FOREIGN KEY (exam_id) REFERENCES exams(id) ON DELETE CASCADE,
  FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='学生成绩表';

-- ============================================
-- 7. 学生小题成绩表（细粒度分数）
-- ============================================
CREATE TABLE IF NOT EXISTS score_items (
  id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '小题成绩ID',
  score_id BIGINT NOT NULL COMMENT '成绩ID（关联scores表）',
  question_number VARCHAR(20) NOT NULL COMMENT '小题编号',
  knowledge_point VARCHAR(100) COMMENT '知识点',
  score FLOAT NOT NULL DEFAULT 0 COMMENT '得分',
  full_score FLOAT NOT NULL DEFAULT 0 COMMENT '总分',
  is_correct TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否正确（0:错误, 1:正确）',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  INDEX idx_score_id (score_id),
  INDEX idx_knowledge_point (knowledge_point),
  FOREIGN KEY (score_id) REFERENCES scores(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='学生小题成绩表';
