package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/BerniceZTT/crm_end/models"
	"github.com/BerniceZTT/crm_end/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func HasProjects(ctx context.Context, customerID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"customerId": customerID,
		"webHidden":  false,
	}

	collection := repository.Collection(repository.ProjectsCollection)

	findOptions := options.Find().SetLimit(1)

	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		log.Printf("查询测试中的项目失败: %v", err)
		return false, fmt.Errorf("查询测试中的项目失败: %v", err)
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err = cursor.All(ctx, &projects); err != nil {
		log.Printf("解析项目失败: %v", err)
		return false, fmt.Errorf("解析项目失败: %v", err)
	}

	return len(projects) > 0, nil
}

func UpdateCustomerProgress(ctx context.Context, customerObjID primitive.ObjectID, progress string) error {
	customersCollection := repository.Collection(repository.CustomersCollection)
	updateData := bson.M{
		"progress":       progress,
		"lastupdatetime": time.Now(),
		"updatedAt":      time.Now(),
	}
	_, err := customersCollection.UpdateOne(
		ctx,
		bson.M{"_id": customerObjID},
		bson.M{"$set": updateData},
	)
	return err
}

func UpdateCustomerProgressByName(ctx context.Context, name string, progress string) error {
	customersCollection := repository.Collection(repository.CustomersCollection)
	updateData := bson.M{
		"progress":       progress,
		"lastupdatetime": time.Now(),
		"updatedAt":      time.Now(),
	}
	_, err := customersCollection.UpdateMany(
		ctx,
		bson.M{"name": name, "progress": models.CustomerProgressInitialContact},
		bson.M{"$set": updateData},
	)
	return err
}
