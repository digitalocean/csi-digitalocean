package driver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/digitalocean/godo"
)

func TestTagger(t *testing.T) {
	tag := "k8s:my-cluster-id"
	tcs := []struct {
		name               string
		vol                *godo.Volume
		createTagFunc      func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)
		tagResourcesFunc   func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error)
		tagExists          bool
		expectCreates      int
		expectTagResources int
		expectError        bool
		expectTags         int
	}{
		{
			name:               "success existing tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectTagResources: 1,
			tagExists:          true,
			expectTags:         1,
		},
		{
			name:               "success with new tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      1,
			expectTagResources: 2,
			expectTags:         1,
		},
		{
			name: "success already tagged",
			vol: &godo.Volume{
				ID:   "hello-world",
				Tags: []string{tag},
			},
			expectCreates:      0,
			expectTagResources: 0,
		},
		{
			name:               "failed first tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      0,
			expectTagResources: 1,
			expectError:        true,
			tagResourcesFunc: func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error) {
				return nil, errors.New("an error")
			},
		},
		{
			name:               "failed create tag",
			vol:                &godo.Volume{ID: "hello-world"},
			expectCreates:      1,
			expectTagResources: 1,
			expectError:        true,
			createTagFunc: func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
				return nil, nil, errors.New("an error")
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			tagService := &fakeTagsDriver{
				createFunc:       tc.createTagFunc,
				tagResourcesFunc: tc.tagResourcesFunc,
				exists:           tc.tagExists,
			}
			driver := &Driver{
				doVolTag: tag,
				tags:     tagService,
			}

			err := driver.tagVolume(context.Background(), tc.vol)

			if err != nil && !tc.expectError {
				t.Errorf("expected success but got error %v", err)
			} else if tc.expectError && err == nil {
				t.Error("expected error but got success")
			}

			if tagService.createCount != tc.expectCreates {
				t.Errorf("createCount was %d, expected %d", tagService.createCount, tc.expectCreates)
			}
			if tagService.tagResourcesCount != tc.expectTagResources {
				t.Errorf("tagResourcesCount was %d, expected %d", tagService.tagResourcesCount, tc.expectTagResources)
			}
			if tc.expectTags != len(tagService.resources) {
				t.Errorf("expected %d tagged volume, %d found", tc.expectTags, len(tagService.resources))
			}
		})
	}
}

type fakeTagsDriver struct {
	createFunc        func(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)
	tagResourcesFunc  func(context.Context, string, *godo.TagResourcesRequest) (*godo.Response, error)
	exists            bool
	resources         []godo.Resource
	createCount       int
	tagResourcesCount int
}

func (*fakeTagsDriver) List(context.Context, *godo.ListOptions) ([]godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (*fakeTagsDriver) Get(context.Context, string) (*godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsDriver) Create(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
	f.createCount++
	if f.createFunc != nil {
		return f.createFunc(ctx, req)
	}
	f.exists = true
	return &godo.Tag{
		Name: req.Name,
	}, godoResponse(), nil
}

func (*fakeTagsDriver) Delete(context.Context, string) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsDriver) TagResources(ctx context.Context, tag string, req *godo.TagResourcesRequest) (*godo.Response, error) {
	f.tagResourcesCount++
	if f.tagResourcesFunc != nil {
		return f.tagResourcesFunc(ctx, tag, req)
	}
	if !f.exists {
		return &godo.Response{
			Response: &http.Response{StatusCode: 404},
			Rate:     godo.Rate{Limit: 10, Remaining: 10},
		}, errors.New("An error occured")
	}
	f.resources = append(f.resources, req.Resources...)
	return godoResponse(), nil
}

func (*fakeTagsDriver) UntagResources(context.Context, string, *godo.UntagResourcesRequest) (*godo.Response, error) {
	panic("not implemented")
}
