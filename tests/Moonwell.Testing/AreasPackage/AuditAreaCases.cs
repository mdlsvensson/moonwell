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

        /// <summary>A rect from (0, 0) to (256, 256).</summary>
        private static AreaShape Square()
        {
            return AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(256f, 256f)));
        }

        /// <summary>Decision 75: a unit inside that Accepts now refuses leaves at the next quarter-second check, with OnUnitLeft, and enters again at the first check after a yes; nothing happens between checks.</summary>
        public static void RecheckLeavesAndEnters(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = Labelled(world, area, "a");
            var allowed = true;
            area.Accept = u => allowed;
            fake.Walk(unit.Handle, 10f, 10f);
            ExpectLog(area, new[] { "entered a" }, "entered");
            allowed = false;
            world.Advance(0.21875f);
            ExpectLog(area, new[] { "entered a" }, "nothing before the check");
            world.Advance(0.03125f);
            ExpectLog(area, new[] { "entered a", "left a" }, "a no at the check: it leaves");
            Expect.True(!area.Has(unit) && area.Units.Count == 0, "it is not inside");
            Expect.True(AreaTracker.Running, "a refused unit in the area keeps the check running");
            world.Advance(0.5f);
            Expect.Equal(2, area.Log.Count, "nothing while the answer is no");
            allowed = true;
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered a", "left a", "entered a" }, "a yes at a later check: it enters again");
            Expect.True(area.Has(unit), "inside again");
            fake.Walk(unit.Handle, 900f, 900f);
            ExpectLog(area, new[] { "entered a", "left a", "entered a", "left a" }, "it walks out");
            Expect.True(!AreaTracker.Running, "nothing inside or refused: the timer stopped");
        }

        /// <summary>A unit Accepts refused as it came inside is asked again at each check while it is in the area, and enters at the first yes; once inside it is asked at each check too.</summary>
        public static void RecheckRefusedAtEntry(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = Labelled(world, area, "a");
            var allowed = false;
            area.Accept = u => allowed;
            fake.Walk(unit.Handle, 10f, 10f);
            Expect.Equal(0, area.Log.Count, "refused");
            Expect.Equal(1, area.AcceptsAsked, "asked once as it came inside");
            world.Advance(0.25f);
            Expect.Equal(2, area.AcceptsAsked, "asked again at the check");
            Expect.Equal(0, area.Log.Count, "still refused");
            allowed = true;
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered a" }, "entered at the check after the yes");
            Expect.Equal(3, area.AcceptsAsked, "asked once at that check");
            world.Advance(0.25f);
            Expect.Equal(4, area.AcceptsAsked, "and once at each check while inside");
            Expect.True(area.Has(unit), "inside");
        }

        /// <summary>With RecheckAccepts off, Accepts is asked only as a unit comes inside: a unit inside stays when the answer turns to no, and a refused one waits for its next enter event.</summary>
        public static void RecheckOff(FakeWorld world)
        {
            var fake = Fresh();
            var area = new LogArea(Square());
            area.SetRecheck(false);
            Added(world, area);
            var first = Labelled(world, area, "first");
            var second = Labelled(world, area, "second");
            var allowed = true;
            area.Accept = u => allowed;
            fake.Walk(first.Handle, 10f, 10f);
            allowed = false;
            fake.Walk(second.Handle, 20f, 20f);
            world.Advance(1f);
            ExpectLog(area, new[] { "entered first" }, "the first stays inside, the second stays out");
            Expect.Equal(2, area.AcceptsAsked, "asked only as each came inside");
            allowed = true;
            world.Advance(1f);
            Expect.True(!area.Has(second), "the second is not asked again by itself");
            fake.Walk(second.Handle, 900f, 900f);
            fake.Walk(second.Handle, 20f, 20f);
            ExpectLog(area, new[] { "entered first", "entered second" }, "the second enters at its next enter event");
            Expect.Equal(3, area.AcceptsAsked, "asked once more");
        }

        /// <summary>A refused unit that walks out is forgotten: it is not asked again, and the timer stops; one that is no longer in the region with no event is forgotten at the next check.</summary>
        public static void RecheckForgetsLeavers(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            area.Accept = u => false;
            var walks = Labelled(world, area, "walks");
            var slips = Labelled(world, area, "slips");
            fake.Walk(walks.Handle, 10f, 10f);
            Expect.True(AreaTracker.Running, "the refused unit keeps the check running");
            fake.Walk(walks.Handle, 900f, 900f);
            Expect.True(!AreaTracker.Running, "it walked out: forgotten, and the timer stopped");
            world.Advance(1f);
            Expect.Equal(1, area.AcceptsAsked, "never asked again");
            fake.Walk(slips.Handle, 20f, 20f);
            fake.Regions[0].Inside.Remove(slips.Handle);
            world.Advance(0.25f);
            Expect.Equal(2, area.AcceptsAsked, "a unit no longer in the region is not asked");
            Expect.True(!AreaTracker.Running, "and is forgotten at the check");
            Expect.Equal(0, area.Log.Count, "nothing entered");
        }

        /// <summary>An Accepts that ends its area at the check lets each unit inside leave once, and nothing runs after.</summary>
        public static void RecheckEndsArea(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 0.1f));
            var a = Labelled(world, area, "a");
            var b = Labelled(world, area, "b");
            fake.Walk(a.Handle, 10f, 10f);
            fake.Walk(b.Handle, 20f, 20f);
            area.Accept = u =>
            {
                area.End();
                return false;
            };
            world.Advance(0.25f);
            Expect.True(area.HasEnded, "the area ended");
            var log = string.Join(", ", area.Log.ToArray());
            Expect.True(log.EndsWith("left a, left b"), "each left once, in entering order: " + log);
            var count = area.Log.Count;
            world.Advance(1f);
            Expect.Equal(count, area.Log.Count, "nothing after");
            Expect.True(!AreaTracker.Running, "the timer stopped");
            Expect.Equal(0, world.Logged.Count, "no error");
        }

        /// <summary>
        /// A corpse in an area with a Tick keeps only the quarter-second check running, not the 1/32 s step: the step runs while
        /// an area with a Tick has a unit inside. Brought back to life with no event (a Resurrection), the unit enters at the
        /// next check and the step runs again.
        /// </summary>
        public static void CorpseChecksWithoutSteps(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 10f, 10f);
            Expect.True(AreaTracker.Running && AreaTracker.Stepping, "a unit inside an area with a Tick: the check and the step");
            fake.NoPlace.Add(unit.Handle);
            fake.Die(unit.Handle);
            ExpectLog(area, new[] { "entered a", "left a" }, "it left when it died");
            Expect.True(AreaTracker.Running, "the corpse keeps the check running");
            Expect.True(!AreaTracker.Stepping, "but not the step");
            world.Advance(1f);
            fake.NoPlace.Remove(unit.Handle);
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered a", "left a", "entered a" }, "brought back, it enters at the next check");
            Expect.True(AreaTracker.Stepping, "and the step runs again");
            world.Advance(1f);
            ExpectLog(area, new[] { "entered a", "left a", "entered a", "stays a" }, "its stay a Tick after it entered");
        }

        /// <summary>An area without a Tick never runs the step: a unit inside keeps only the check running, which still takes out a unit hidden inside.</summary>
        public static void NoTickNoSteps(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 10f, 10f);
            Expect.True(AreaTracker.Running && !AreaTracker.Stepping, "the check only");
            var owned = Game.Instance.OwnedCount;
            fake.NoPlace.Add(unit.Handle);
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered a", "left a" }, "the hidden unit left at the check");
            fake.NoPlace.Remove(unit.Handle);
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered a", "left a", "entered a" }, "and entered when shown");
            Expect.Equal(owned, Game.Instance.OwnedCount, "one timer all along");
        }

        /// <summary>Many corpses in an area are checked in the order they died, each found again when brought back, and forgotten once out of the region.</summary>
        public static void ManyCorpses(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var units = new List<Unit>();
            for (var i = 0; i < 60; i++)
            {
                var unit = Labelled(world, area, "u" + i);
                units.Add(unit);
                fake.Walk(unit.Handle, 10f + i, 10f);
            }

            for (var i = 0; i < units.Count; i++)
            {
                fake.NoPlace.Add(units[i].Handle);
                fake.Die(units[i].Handle);
            }

            Expect.Equal(120, area.Log.Count, "each entered and left");
            world.Advance(1f);
            fake.NoPlace.Remove(units[59].Handle);
            fake.NoPlace.Remove(units[3].Handle);
            world.Advance(0.25f);
            Expect.Equal("entered u3, entered u59", area.Log[120] + ", " + area.Log[121], "the two brought back enter, in the order they died");
            fake.Walk(units[3].Handle, 900f, 900f);
            fake.Walk(units[59].Handle, 900f, 900f);
            Expect.True(AreaTracker.Running, "the other corpses keep the check running");
            for (var i = 0; i < units.Count; i++)
            {
                if (i != 3 && i != 59)
                {
                    units[i].Kill();
                    fake.Regions[0].Inside.Remove(units[i].Handle);
                }
            }

            world.Advance(0.25f);
            Expect.True(!AreaTracker.Running, "the corpses out of the region are forgotten, and the timer stopped");
        }

        /// <summary>What OnUnitLeft makes while its area ends: a timer belongs to the area and ends at once (decision 11); a component added to the unit is the unit's (decision 65) and stays; a timer made under Game.Instance.Run outlives the area.</summary>
        public static void LeftWhileEnding(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = Labelled(world, area, "a");
            fake.Walk(unit.Handle, 10f, 10f);
            Timer? own = null;
            Timer? kept = null;
            Tally? tally = null;
            var fired = 0;
            area.WhenLeft = u =>
            {
                own = Timer.After(1f, () => fired++);
                Game.Instance.Run(() => kept = Timer.After(1f, () => fired += 10));
                tally = u.Add(new Tally());
            };
            area.End();
            ExpectLog(area, new[] { "entered a", "left a" }, "it left as the area ended");
            Expect.True(own!.HasEnded, "the area's timer ended with it");
            Expect.True(!kept!.HasEnded, "the game's did not");
            Expect.True(!tally!.HasEnded && unit.Has<Tally>(), "the component is the unit's");
            world.Advance(1f);
            Expect.Equal(10, fired, "only the game's timer fired");
        }

        /// <summary>A map's area with fields of its own named as Area's keeps them apart from the base's: the area enters, stays and leaves its unit, and its own fields keep their values. CSharp.lua gives the subclass's fields names of their own (inside1), as it sees Area's private ones in the same compile.</summary>
        public static void ShadowedFields(FakeWorld world)
        {
            var fake = Fresh();
            var area = new ShadowArea(AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(256f, 256f))));
            Game.Instance.Add(area);
            world.Advance(0f);
            Expect.Equal("own shape, 7, 0, 0, 0, idle, 0", area.Own(), "its own fields at the start");
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("hfoo")))!;
            fake.Walk(unit.Handle, 10f, 10f);
            Expect.True(area.Has(unit) && area.Units.Count == 1, "the unit is inside");
            Expect.True(area.Contains(10f, 10f) && !area.Contains(300f, 10f), "the shape is the area's");
            world.Advance(1f);
            Expect.True(AreaTracker.Running, "the timer runs");
            fake.Walk(unit.Handle, 900f, 900f);
            Expect.Equal("entered, stays, left", string.Join(", ", area.Log.ToArray()), "the reactions");
            Expect.Equal("own shape, 7, 1, 1, 1, idle, 1", area.Own(), "its own fields");
            Expect.True(!AreaTracker.Running, "the timer stopped with the last unit");
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
