using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for AreaShape and the cells the game is given for a polygon or a circle.</summary>
    public static class AreaShapeCases
    {
        /// <summary>Installs the package on a fake world with one region, "Shop", from (0, 0) to (256, 128).</summary>
        private static void InstallShop()
        {
            AreaSystem.Install(new FakeAreaWorld(), new[] { "Shop" }, new[] { 0f, 0f, 256f, 128f });
        }

        /// <summary>Whether a list of centres holds a cell's centre.</summary>
        private static bool Has(List<float> centres, float x, float y)
        {
            for (var i = 0; i < centres.Count; i += 2)
            {
                if (centres[i] == x && centres[i + 1] == y)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>A region by name is the World Editor rect, edges included.</summary>
        public static void Named(FakeWorld world)
        {
            InstallShop();
            var shape = AreaShape.Region("Shop");
            Expect.True(shape.Bounds == new Bounds(new Vector2(0f, 0f), new Vector2(256f, 128f)), "the region's bounds");
            Expect.True(shape.Contains(256f, 128f), "the top right corner is inside");
            Expect.True(shape.Contains(new Vector2(0f, 0f)), "the bottom left corner is inside");
            Expect.True(!shape.Contains(257f, 0f), "one past the right edge is outside");
            var cells = new List<float>();
            AreaCells.Of(shape, cells);
            Expect.Equal(0, cells.Count, "a region goes to the game as a rect, so it adds no cells");
        }

        /// <summary>A name the map has no region for throws at the first read, not when the shape is made, and says the name.</summary>
        public static void UnknownName(FakeWorld world)
        {
            InstallShop();
            var shape = AreaShape.Region("Bank");
            Expect.Throws<ArgumentException>(() => { var unused = shape.Bounds; }, "an unknown name, at the first read");
            Expect.Throws<ArgumentException>(() => shape.Contains(1f, 1f), "and at every read");
            try
            {
                var bounds = shape.Bounds;
            }
            catch (ArgumentException error)
            {
                Expect.Equal("No World Editor region is named 'Bank'.", error.Message, "the message");
            }

            var union = AreaShape.Of(AreaShape.Region("Shop"), shape);
            Expect.Throws<ArgumentException>(() => { var unused = union.Bounds; }, "a union with an unknown region");
        }

        /// <summary>A circle is the cells whose centre is within the radius; Contains is the true circle.</summary>
        public static void Circle(FakeWorld world)
        {
            var small = AreaShape.Circle(new Vector2(0f, 0f), 48f);
            var cells = new List<float>();
            AreaCells.Of(small, cells);
            Expect.Equal(8, cells.Count, "radius 48 holds 4 cells");
            Expect.True(Has(cells, -16f, -16f) && Has(cells, 16f, -16f) && Has(cells, -16f, 16f) && Has(cells, 16f, 16f), "the four centres are (+-16, +-16)");
            Expect.True(!small.Contains(50f, 0f), "50 is outside a radius of 48");
            Expect.True(small.Contains(48f, 0f), "the radius itself is inside");
            Expect.True(small.Bounds == new Bounds(new Vector2(-48f, -48f), new Vector2(48f, 48f)), "the circle's bounds");

            var wider = AreaShape.Circle(new Vector2(0f, 0f), 51f);
            var more = new List<float>();
            AreaCells.Of(wider, more);
            Expect.Equal(24, more.Count, "radius 51 holds 12 cells");
            Expect.True(Has(more, 48f, 16f) && Has(more, -48f, -16f) && Has(more, 16f, 48f) && Has(more, -16f, -48f), "it adds (+-48, +-16) and (+-16, +-48)");
            Expect.True(!Has(more, 48f, 48f), "the corner cells stay out");
            Expect.Near(-16f, more[0], "the first cell is the bottom row, left of the two: x");
            Expect.Near(-48f, more[1], "the first cell is the bottom row: y");
            Expect.Throws<ArgumentException>(() => AreaShape.Circle(new Vector2(0f, 0f), 0f), "radius 0");
            Expect.Throws<ArgumentException>(() => AreaShape.Circle(new Vector2(0f, 0f), -5f), "a negative radius");
        }

        /// <summary>A polygon is the cells whose centre is inside it, row by row from the bottom left; Contains is Polygon.Contains.</summary>
        public static void Polygon(FakeWorld world)
        {
            var triangle = new Polygon(new Vector2(0f, 0f), new Vector2(130f, 0f), new Vector2(0f, 130f));
            var shape = AreaShape.Polygon(triangle);
            var cells = new List<float>();
            AreaCells.Of(shape, cells);
            Expect.Equal(20, cells.Count, "10 cells");
            for (var i = 0; i < cells.Count; i += 2)
            {
                Expect.True(cells[i] + cells[i + 1] < 130f, "a cell centre is inside the triangle");
            }

            Expect.True(Has(cells, 16f, 16f) && Has(cells, 112f, 16f) && Has(cells, 16f, 112f), "the corners of the staircase");
            Expect.True(!Has(cells, 112f, 48f), "112 + 48 is past the edge");
            Expect.Near(16f, cells[0], "the first cell: x");
            Expect.Near(16f, cells[1], "the first cell: y");
            Expect.Near(48f, cells[2], "the second cell is in the same row: x");
            Expect.Near(16f, cells[3], "the second cell is in the same row: y");
            Expect.True(shape.Bounds == triangle.Bounds, "the polygon's bounds");
            var xs = new[] { -5f, 0f, 10f, 64f, 65f, 66f, 100f, 130f };
            for (var i = 0; i < xs.Length; i++)
            {
                for (var j = 0; j < xs.Length; j++)
                {
                    Expect.True(shape.Contains(xs[i], xs[j]) == triangle.Contains(xs[i], xs[j]), "Contains matches Polygon.Contains at " + xs[i] + ", " + xs[j]);
                }
            }

            Expect.True(shape.Contains(new Vector2(10f, 10f)), "the vector form");
        }

        /// <summary>Of covers all its shapes: the bounds hold both, a point in either is inside, and the cells are those of its parts.</summary>
        public static void Of(FakeWorld world)
        {
            var left = AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(64f, 64f)));
            var right = AreaShape.Rect(new Bounds(new Vector2(200f, 100f), new Vector2(300f, 160f)));
            var both = AreaShape.Of(left, right);
            Expect.True(both.Bounds == new Bounds(new Vector2(0f, 0f), new Vector2(300f, 160f)), "the bounds cover both");
            Expect.True(both.Contains(10f, 10f), "a point in the first");
            Expect.True(both.Contains(250f, 120f), "a point in the second");
            Expect.True(!both.Contains(150f, 80f), "a point between them");
            var cells = new List<float>();
            AreaCells.Of(both, cells);
            Expect.Equal(0, cells.Count, "two rects add no cells");

            var mixed = AreaShape.Of(left, AreaShape.Circle(new Vector2(192f, 192f), 48f));
            var circleCells = new List<float>();
            AreaCells.Of(mixed, circleCells);
            Expect.Equal(8, circleCells.Count, "the circle's 4 cells and nothing from the rect");
            Expect.Throws<ArgumentException>(() => AreaShape.Of(), "no shapes");
        }

        /// <summary>A region shape may be made before the package is installed; reading its members then throws InvalidOperationException, and after Install it reads.</summary>
        public static void NotInstalled(FakeWorld world)
        {
            AreaSystem.Uninstall();
            var shape = AreaShape.Region("Shop");
            var union = AreaShape.Of(shape, AreaShape.Rect(new Bounds(new Vector2(512f, 0f), new Vector2(600f, 64f))));
            Expect.Throws<InvalidOperationException>(() => { var unused = shape.Bounds; }, "Bounds before Install");
            Expect.Throws<InvalidOperationException>(() => shape.Contains(1f, 1f), "Contains before Install");
            Expect.Throws<InvalidOperationException>(() => { var unused = union.Bounds; }, "a union holding a region, before Install");
            InstallShop();
            Expect.True(shape.Contains(1f, 1f), "after Install it reads");
            Expect.True(shape.Bounds == new Bounds(new Vector2(0f, 0f), new Vector2(256f, 128f)), "its bounds");
            Expect.True(union.Bounds == new Bounds(new Vector2(0f, 0f), new Vector2(600f, 128f)), "a union holding it resolves the same way");
            Expect.True(union.Contains(550f, 10f) && union.Contains(10f, 10f), "and holds both parts");
        }

        /// <summary>The install refuses rects that are not four floats a name.</summary>
        public static void WrongLength(FakeWorld world)
        {
            InstallShop();
            Expect.Throws<ArgumentException>(() => AreaSystem.Install(new FakeAreaWorld(), new[] { "Shop", "Bank" }, new[] { 0f, 0f, 256f, 128f }), "two names, four floats");
            Expect.True(AreaShape.Region("Shop").Contains(1f, 1f), "the earlier install is kept");
        }
    }
}
