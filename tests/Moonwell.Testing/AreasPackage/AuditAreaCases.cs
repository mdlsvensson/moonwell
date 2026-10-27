using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to areas: the cells the game is given, and what the place check asks.</summary>
    public static class AuditAreaCases
    {
        /// <summary>Installs the package on a fake world with no World Editor regions and returns it.</summary>
        private static FakeAreaWorld Fresh()
        {
            var fake = new FakeAreaWorld();
            AreaSystem.Install(fake, new string[0], new float[0]);
            return fake;
        }

        /// <summary>The fake's regions cover points as the game's do (game facts, 10-25 and 10-26): a cell from the multiple of 32 below to the next, and a rect from the cell of its min corner to the cell of its max corner, so a max edge on the 32 grid takes the cell past it.</summary>
        public static void FakeCoversAsTheGame(FakeWorld world)
        {
            var fake = new FakeAreaWorld();
            var square = fake.NewRegion();
            fake.AddRect(square, 0f, 0f, 256f, 256f);
            var made = fake.Of(square);
            Expect.True(fake.Covers(made, 0f, 100f) && fake.Covers(made, 256f, 100f) && fake.Covers(made, 287.9f, 100f), "(0, 0)-(256, 256) covers 0, 256 and 287.9");
            Expect.True(!fake.Covers(made, -0.1f, 100f) && !fake.Covers(made, 288f, 100f), "and not -0.1 or 288");
            var odd = fake.NewRegion();
            fake.AddRect(odd, 100.5f, 100.5f, 300.5f, 300.5f);
            Expect.True(fake.Covers(fake.Of(odd), 96f, 319.9f) && !fake.Covers(fake.Of(odd), 95.9f, 200f) && !fake.Covers(fake.Of(odd), 200f, 320f), "(100.5, 100.5)-(300.5, 300.5) covers 96 to 320");
            var cells = fake.NewRegion();
            fake.AddCell(cells, 10f, 10f);
            fake.AddCell(cells, -10f, -10f);
            Expect.True(fake.Covers(fake.Of(cells), 0f, 31.9f) && !fake.Covers(fake.Of(cells), 32f, 10f), "the cell at (10, 10) covers 0 to 32");
            Expect.True(fake.Covers(fake.Of(cells), -32f, -0.1f) && !fake.Covers(fake.Of(cells), -32.1f, -10f), "the cell at (-10, -10) covers -32 to 0");
        }

        /// <summary>Makes a unit of the game, labelled for an area's log.</summary>
        private static Unit Labelled(FakeWorld world, LogArea area, string label)
        {
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            area.Labels[unit] = label;
            return unit;
        }

        /// <summary>Adds an area to the game, lets its first, 0 second timer run, and returns it.</summary>
        private static LogArea Added(FakeWorld world, LogArea area)
        {
            Game.Instance.Add(area);
            world.Advance(0f);
            return area;
        }

        /// <summary>Fails unless the area's log is exactly these lines.</summary>
        private static void ExpectLog(LogArea area, string[] expected, string what)
        {
            Expect.Equal(string.Join(", ", expected), string.Join(", ", area.Log.ToArray()), what);
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

        /// <summary>A circle of radius 20 at a cell corner holds no cell's centre; it still has the cell its own centre is in (decision 67), and a unit walking into that cell, inside the circle, enters.</summary>
        public static void SmallCircle(FakeWorld world)
        {
            var fake = Fresh();
            var mine = AreaShape.Circle(new Vector2(96f, 96f), 20f);
            var cells = new List<float>();
            AreaCells.Of(mine, cells);
            Expect.Equal(2, cells.Count, "one cell");
            Expect.True(Has(cells, 112f, 112f), "the cell holding (96, 96), 96 to 128");
            var area = Added(world, new LogArea(mine));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 300f, 300f);
            fake.Walk(unit.Handle, 100f, 100f);
            Expect.True(area.Contains(100f, 100f), "(100, 100) is inside the circle");
            ExpectLog(area, new[] { "entered a" }, "the unit in the centre's cell enters");
            fake.Walk(unit.Handle, 300f, 300f);
            ExpectLog(area, new[] { "entered a", "left a" }, "and leaves");
        }

        /// <summary>A diagonal band 28 wide holds no cell's centre: it still has the cell its centroid is in, and a unit there enters.</summary>
        public static void ThinPolygon(FakeWorld world)
        {
            var fake = Fresh();
            var band = AreaShape.Polygon(new Polygon(new Vector2(0f, 2f), new Vector2(1000f, 1002f), new Vector2(1000f, 1030f), new Vector2(0f, 30f)));
            var cells = new List<float>();
            AreaCells.Of(band, cells);
            Expect.Equal(2, cells.Count, "one cell");
            Expect.True(Has(cells, 496f, 528f), "the cell holding the centroid (500, 516)");
            var area = Added(world, new LogArea(band));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 500f, 516f);
            Expect.True(area.Contains(500f, 516f), "the centroid is inside the band");
            ExpectLog(area, new[] { "entered a" }, "the unit at the centroid enters");
            Expect.Equal(0, world.Logged.Count, "no error");
        }

        /// <summary>A band with cells along its diagonal does not hold the centre of the cell its centroid is in; that cell is added all the same, after the others.</summary>
        public static void CentreCellAlways(FakeWorld world)
        {
            var fake = Fresh();
            var band = AreaShape.Polygon(new Polygon(new Vector2(20f, 10f), new Vector2(1000f, 990f), new Vector2(1000f, 1020f), new Vector2(20f, 40f)));
            var cells = new List<float>();
            AreaCells.Of(band, cells);
            Expect.True(Has(cells, 48f, 48f) && Has(cells, 976f, 976f), "the cells along the diagonal");
            Expect.True(!band.Contains(496f, 528f), "the centre of the centroid's cell is outside the band");
            Expect.True(Has(cells, 496f, 528f), "and the cell is added");
            Expect.Near(496f, cells[cells.Count - 2], "last: x");
            Expect.Near(528f, cells[cells.Count - 1], "last: y");
            var area = Added(world, new LogArea(band));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 510f, 515f);
            ExpectLog(area, new[] { "entered a" }, "a unit at the centroid enters");
        }

        /// <summary>A unit walked into a region gets an enter event, and out of it a leave event, once each.</summary>
        public static void FakeWalks(FakeWorld world)
        {
            var fake = Fresh();
            var area = new LogArea(AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(100f, 100f))));
            Game.Instance.Add(area);
            world.Advance(0f);
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            area.Labels[unit] = "a";
            fake.Walk(unit.Handle, 500f, 500f);
            fake.Walk(unit.Handle, 50f, 50f);
            fake.Walk(unit.Handle, 60f, 60f);
            fake.Walk(unit.Handle, 500f, 50f);
            Expect.Equal("entered a, left a", string.Join(", ", area.Log.ToArray()), "one enter and one leave");
        }
    }
}
