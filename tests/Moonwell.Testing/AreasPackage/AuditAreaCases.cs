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

        /// <summary>
        /// Two rect areas that share an edge at x 256 do not overlap: the game would give a rect whose max edge is on the 32
        /// grid the cell past it, so the left one is given its cells to 256 and no further, and a unit at x 270 is in the
        /// right one only. A World Editor region is given its cells the same way.
        /// </summary>
        public static void SharedEdge(FakeWorld world)
        {
            var fake = new FakeAreaWorld();
            AreaSystem.Install(fake, new[] { "West", "East" }, new[] { 0f, 0f, 256f, 128f, 256f, 0f, 512f, 128f });
            var left = Added(world, new LogArea(AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(256f, 256f)))));
            var right = Added(world, new LogArea(AreaShape.Rect(new Bounds(new Vector2(256f, 0f), new Vector2(512f, 256f)))));
            var west = Added(world, new LogArea(AreaShape.Region("West")));
            var east = Added(world, new LogArea(AreaShape.Region("East")));
            var leftCells = fake.Regions[0];
            Expect.True(fake.Covers(leftCells, 0f, 0f) && fake.Covers(leftCells, 255.9f, 255.9f), "the left area covers 0 to 256");
            Expect.True(!fake.Covers(leftCells, 256f, 100f) && !fake.Covers(leftCells, 287f, 100f) && !fake.Covers(leftCells, 100f, 256f), "and not the cells past its max edges");
            Expect.True(fake.Covers(fake.Regions[1], 256f, 100f) && fake.Covers(fake.Regions[1], 511.9f, 255.9f) && !fake.Covers(fake.Regions[1], 512f, 100f), "the right one covers 256 to 512");
            Expect.True(!fake.Covers(fake.Regions[2], 256f, 100f) && fake.Covers(fake.Regions[2], 255.9f, 127.9f) && !fake.Covers(fake.Regions[2], 100f, 128f), "the region West covers 0 to 256 and 0 to 128");
            Expect.True(fake.Covers(fake.Regions[3], 256f, 100f), "and East from 256");
            var unit = Labelled(world, left, "u");
            right.Labels[unit] = "u";
            west.Labels[unit] = "u";
            east.Labels[unit] = "u";
            fake.Walk(unit.Handle, 270f, 100f);
            ExpectLog(left, new string[0], "at x 270 the unit is not in the left area");
            ExpectLog(right, new[] { "entered u" }, "it is in the right one");
            ExpectLog(west, new string[0], "nor in West");
            ExpectLog(east, new[] { "entered u" }, "it is in East");
            fake.Walk(unit.Handle, 250f, 100f);
            ExpectLog(left, new[] { "entered u" }, "at x 250 it is in the left area");
            ExpectLog(right, new[] { "entered u", "left u" }, "and has left the right one");
            ExpectLog(west, new[] { "entered u" }, "it is in West");
            ExpectLog(east, new[] { "entered u", "left u" }, "and has left East");
            Expect.True(left.Contains(256f, 100f) && right.Contains(256f, 100f), "Contains has the shared edge in both, as the true shapes do");
        }

        /// <summary>A rect whose edges are off the grid is still rounded out to every cell it touches, and a rect with no width has its one column of cells.</summary>
        public static void RectRoundedOut(FakeWorld world)
        {
            var fake = Fresh();
            Added(world, new LogArea(AreaShape.Rect(new Bounds(new Vector2(100.5f, -100f), new Vector2(300.5f, -64f)))));
            Added(world, new LogArea(AreaShape.Rect(new Bounds(new Vector2(64f, 0f), new Vector2(64f, 64f)))));
            var odd = fake.Regions[0];
            Expect.True(fake.Covers(odd, 96f, -128f) && fake.Covers(odd, 319.9f, -64.1f), "(100.5, -100)-(300.5, -64) covers 96 to 320 and -128 to -64");
            Expect.True(!fake.Covers(odd, 95.9f, -100f) && !fake.Covers(odd, 320f, -100f) && !fake.Covers(odd, 200f, -64f) && !fake.Covers(odd, 200f, -128.1f), "and no further");
            var line = fake.Regions[1];
            Expect.True(fake.Covers(line, 64f, 10f) && fake.Covers(line, 95.9f, 63.9f), "a rect with no width covers the cells at its x");
            Expect.True(!fake.Covers(line, 63.9f, 10f) && !fake.Covers(line, 96f, 10f) && !fake.Covers(line, 64f, 64f), "and no others");
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
