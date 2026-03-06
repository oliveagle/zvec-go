// Example: Testing real zvec C++ CGO binding
package main

import (
	"fmt"
	"log"

	"github.com/oliveagle/zvec-go/zvec"
)

func main() {
	log.Println("=== zvec CGO 绑定测试 ===")

	// 初始化 zvec
	config := &zvec.Config{
		LogType:     zvec.LogTypeConsole,
		LogLevel:    zvec.LogLevelInfo,
	}

	if err := zvec.Init(config); err != nil {
		log.Fatalf("Failed to initialize zvec: %v", err)
	}

	defer zvec.CloseInstance()

	// 创建测试 schema
	schema := zvec.NewCollectionSchema("test_cgo")
	idField := zvec.NewFieldSchema("id", zvec.DataTypeInt64)
	vecField := zvec.NewVectorSchema("embedding", zvec.DataTypeVectorFP32, 128)

	schema.AddField(idField)
	vecField = vecField.WithMetricType(zvec.MetricTypeCOSINE)
	schema.AddVectorField(vecField)

	log.Println("Schema created")

	// 创建集合路径
	testPath := "/tmp/zvec_cgo_test"

	// 删除已存在的测试目录
	// if err := os.RemoveAll(testPath); err != nil {
	// 	log.Printf("Warning: Failed to clean test dir: %v", err)
	// }

	// 创建测试集合
	// 注意：这里应该失败，因为没有真实的 C++ 库
	// 但我们需要验证 CGO 绑定的代码是否正确

	log.Println("\n--- 尝试创建集合 ---\n")

	coll, err := zvec.CreateAndOpen(testPath, schema, nil)
	if err != nil {
		log.Printf("❌ CreateAndOpen 失败（预期行为）: %v", err)
	} else {
		log.Printf("✓ CreateAndOpen 成功: %v", coll)
		log.Println("  集合指针: ", coll)
		log.Println("  集合路径: ", coll.Path())

		// 尝试插入文档
		doc := zvec.NewDocument("doc_001")
		doc.SetField("content", "Hello from real zvec core via CGO!")

		log.Println("\n--- 尝试插入文档 ---\n")
		if err := coll.Insert(doc); err != nil {
			log.Printf("❌ Insert 失败: %v", err)
		} else {
			log.Printf("✓ Insert 成功: %v", err)
		}

		// 尝试获取统计
		log.Println("\n--- 尝试获取统计 ---\n")
		stats, err := coll.Stats()
		if err != nil {
			log.Printf("❌ Stats 失败: %v", err)
		} else {
			log.Printf("✓ Stats 成功: DocCount=%d", stats.DocCount)
		}

		// 尝试关闭集合
		log.Println("\n--- 尝试关闭集合 ---\n")
		if err := coll.Close(); err != nil {
			log.Printf("❌ Close 失败: %v", err)
		} else {
			log.Println("✓ Close 成功")
		}

		// 清理
		if err := os.RemoveAll(testPath); err != nil {
			log.Printf("Warning: Failed to clean test dir: %v", err)
		}

	log.Println("\n=== 测试总结 ===")
	log.Println("zvec C++ 库状态: .so 动态库 或 .a 静态库")
	log.Println("预期：CreateAndOpen 应该成功（如果有 C++ 库）")
	log.Println("预期：Insert 应该成功")
	log.Println("预期：Stats 应该成功")
	log.Println("预期：Close 应该成功")

	// 如果所有操作都失败，说明 CGO 绑定没有正确连接到 C++ 库
}
