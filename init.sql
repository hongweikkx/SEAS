-- 创建数据库
CREATE DATABASE IF NOT EXISTS seas DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;

-- 使用数据库
USE seas;

-- 学生表
CREATE TABLE IF NOT EXISTS `students` (
   `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
   `student_number` VARCHAR(64) NOT NULL UNIQUE,
   `name` VARCHAR(100) NOT NULL,
   `class` VARCHAR(100),
   `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 考试表
CREATE TABLE IF NOT EXISTS `exams` (
   `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
   `name` VARCHAR(100) NOT NULL, -- 例如：2025年春季期中考试
   `exam_date` DATE NOT NULL,
   `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 学科表
CREATE TABLE IF NOT EXISTS `subjects` (
   `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
   `name` VARCHAR(64) NOT NULL UNIQUE
);

-- 成绩总表（某学生某场考试某学科的总成绩）
CREATE TABLE IF NOT EXISTS `scores` (
   `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
   `student_id` BIGINT NOT NULL,
   `exam_id` BIGINT NOT NULL,
   `subject_id` BIGINT NOT NULL,
   `total_score` DECIMAL(5,2) NOT NULL,
   `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
   FOREIGN KEY (`student_id`) REFERENCES `students` (`id`) ON DELETE CASCADE,
   FOREIGN KEY (`exam_id`) REFERENCES `exams` (`id`) ON DELETE CASCADE,
   FOREIGN KEY (`subject_id`) REFERENCES `subjects` (`id`) ON DELETE CASCADE
);

-- 小题得分表（明细）
CREATE TABLE IF NOT EXISTS `score_items` (
   `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
   `score_id` BIGINT NOT NULL, -- 所属总成绩记录
   `question_number` VARCHAR(20) NOT NULL, -- 第几题
   `knowledge_point` VARCHAR(255), -- 知识点标签，如”函数单调性”
   `score` DECIMAL(5,2) NOT NULL,
   `full_score` DECIMAL(5,2) NOT NULL,
   `is_correct` BOOLEAN NOT NULL,
   FOREIGN KEY (`score_id`) REFERENCES `scores` (`id`) ON DELETE CASCADE
);