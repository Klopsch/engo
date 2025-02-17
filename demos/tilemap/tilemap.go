//+build demo

package main

import (
	"image/color"
	"log"

	"github.com/klopsch/ecs"
	"github.com/klopsch/engo"
	"github.com/klopsch/engo/common"
)

type GameWorld struct{}

type Tile struct {
	ecs.BasicEntity
	common.AnimationComponent
	common.RenderComponent
	common.SpaceComponent
}

func (game *GameWorld) Preload() {
	// A tmx file can be generated from the Tiled Map Editor.
	// The engo tmx loader only accepts tmx files that are base64 encoded and compressed with zlib.
	// When you add tilesets to the Tiled Editor, the location where you added them from is where the engo loader will look for them
	// Tileset from : http://opengameart.org

	if err := engo.Files.Load("example.tmx"); err != nil {
		panic(err)
	}
}

func (game *GameWorld) Setup(u engo.Updater) {
	w, _ := u.(*ecs.World)

	common.SetBackground(color.White)

	w.AddSystem(&common.RenderSystem{})
	w.AddSystem(&common.AnimationSystem{})

	resource, err := engo.Files.Resource("example.tmx")
	if err != nil {
		panic(err)
	}
	tmxResource := resource.(common.TMXResource)
	levelData := tmxResource.Level

	// Create render and space components for each of the tiles in all layers
	tileComponents := make([]*Tile, 0)

	for idx, tileLayer := range levelData.TileLayers {
		for _, tileElement := range tileLayer.Tiles {
			if tileElement.Image != nil {

				tile := &Tile{BasicEntity: ecs.NewBasic()}
				if len(tileElement.Drawables) > 0 {
					tile.AnimationComponent = common.NewAnimationComponent(
						tileElement.Drawables, 0.5,
					)
					tile.AnimationComponent.AddDefaultAnimation(tileElement.Animation)
				}
				tile.RenderComponent = common.RenderComponent{
					Drawable:    tileElement.Image,
					Scale:       engo.Point{X: 1, Y: 1},
					StartZIndex: float32(idx),
				}
				tile.SpaceComponent = common.SpaceComponent{
					Position: tileElement.Point,
					Width:    0,
					Height:   0,
				}

				tileComponents = append(tileComponents, tile)
			}
		}
	}

	// Do the same for all image layers
	for _, imageLayer := range levelData.ImageLayers {
		for _, imageElement := range imageLayer.Images {
			if imageElement.Image != nil {
				tile := &Tile{BasicEntity: ecs.NewBasic()}
				tile.RenderComponent = common.RenderComponent{
					Drawable: imageElement,
					Scale:    engo.Point{X: 1, Y: 1},
				}
				tile.SpaceComponent = common.SpaceComponent{
					Position: imageElement.Point,
					Width:    0,
					Height:   0,
				}

				tileComponents = append(tileComponents, tile)
			}
		}
	}

	// Add each of the tiles entities and its components to the render system
	for _, system := range w.Systems() {
		switch sys := system.(type) {
		case *common.RenderSystem:
			for _, v := range tileComponents {
				sys.Add(&v.BasicEntity, &v.RenderComponent, &v.SpaceComponent)
			}
		case *common.AnimationSystem:
			for _, v := range tileComponents {
				sys.Add(&v.BasicEntity, &v.AnimationComponent, &v.RenderComponent)
			}
		}
	}

	// Access Object Layers
	for _, objectLayer := range levelData.ObjectLayers {
		log.Println("This object layer is called " + objectLayer.Name)
		// Do something with every regular Object
		for _, object := range objectLayer.Objects {
			log.Println("This object is called " + object.Name)
		}
	}

}

func (game *GameWorld) Type() string { return "GameWorld" }

func main() {
	opts := engo.RunOptions{
		Title:  "TileMap Demo",
		Width:  800,
		Height: 800,
	}
	engo.Run(opts, &GameWorld{})
}
