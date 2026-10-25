using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for Find, on the default settings but cells of 256 over a 2,048 by 2,048 world from (-1024, -1024), a slack of 64, a step of 1/32 second and a cell check of 0.1 second.</summary>
    public static class FindCases
    {
        /// <summary>Installs a fake interactions world with cells of 256, which the expected cells of these cases count in; the default is 128.</summary>
        private static FakeInteractionWorld Fresh()
        {
            var fake = new FakeInteractionWorld();
            Interactions.Install(fake);
            Interactions.CellSize = 256.0f;
            return fake;
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>Makes a unit of the game at a place and takes it in.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Makes an item of the game at a place, on the world's list of items, and takes it in.</summary>
        private static Item ItemAt(FakeWorld world, float x, float y)
        {
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, x, y);
            world.PlacedItems.Add(handle);
            return Item.Of(handle)!;
        }

        /// <summary>Whether a list holds a thing.</summary>
        private static bool Holds<T>(List<T> list, T thing) where T : Actor
        {
            for (var i = 0; i < list.Count; i++)
            {
                if (list[i] == thing)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Units 299, 300 and 301 from a point: the first two are within a range of 300, in the order they came, by floats and by a vector; the one at 301 is not, and a range below 0 finds nothing.</summary>
        public static void InRange(FakeWorld world)
        {
            Fresh();
            var near = UnitAt(world, 299.0f, 0.0f);
            var edge = UnitAt(world, 0.0f, 300.0f);
            var far = UnitAt(world, -301.0f, 0.0f);
            var found = Find.InRange<Unit>(0.0f, 0.0f, 300.0f);
            Expect.Equal(2, found.Count, "299 and 300 are in, 301 is out");
            Expect.Same(near, found[0], "the unit at 299 first: the grid reads its row, y 0, before the row of the other");
            Expect.Same(edge, found[1], "the unit at 300, at the range exactly");
            Expect.True(!Holds(found, far), "the unit at 301 is out");
            var byVector = Find.InRange<Unit>(new Vector2(0.0f, 0.0f), 300.0f);
            Expect.Equal(2, byVector.Count, "the vector form finds the same");
            Expect.Same(near, byVector[0], "in the same order");
            Expect.Equal(0, Find.InRange<Unit>(0.0f, 0.0f, -1.0f).Count, "a range below 0 finds nothing");
            Expect.Equal(0, Find.InRange<Unit>(0.0f, 0.0f, 0.0f).Count, "a range of 0 finds only what is on the point, here nothing");
        }

        /// <summary>The class asks for what it covers: Actor gives a unit, an item and a gizmo, Unit the unit, Item the item, Well the well; a kind nobody asked for before is tracked by the ask.</summary>
        public static void Kinds(FakeWorld world)
        {
            Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            var item = ItemAt(world, 20.0f, 20.0f);
            var well = Gizmo.Create(new Well(), 30.0f, 30.0f);
            var all = Find.InRange<Actor>(0.0f, 0.0f, 100.0f);
            Expect.Equal(3, all.Count, "Actor gives all three");
            Expect.True(Holds(all, unit) && Holds(all, item) && Holds(all, well), "the unit, the item and the gizmo");
            var units = Find.InRange<Unit>(0.0f, 0.0f, 100.0f);
            Expect.Equal(1, units.Count, "Unit gives the unit only");
            Expect.Same(unit, units[0], "the unit");
            var items = Find.InRange<Item>(0.0f, 0.0f, 100.0f);
            Expect.Equal(1, items.Count, "Item gives the item only");
            Expect.Same(item, items[0], "the item");
            var wells = Find.InRange<Well>(0.0f, 0.0f, 100.0f);
            Expect.Equal(1, wells.Count, "Well gives the well only");
            Expect.Same(well, wells[0], "the well");
            Expect.Equal(1, Find.InRange<Gizmo>(0.0f, 0.0f, 100.0f).Count, "and so does Gizmo");
        }

        /// <summary>A class with no place is an error in a development build, and finds nothing.</summary>
        public static void NoPlaceClass(FakeWorld world)
        {
            Fresh();
            UnitAt(world, 0.0f, 0.0f);
#if DEBUG
            Expect.Throws<MoonwellException>(() => Find.InRange<Player>(0.0f, 0.0f, 100.0f), "Player has no place");
#endif
        }

        /// <summary>The filter is asked of what is inside the shape and of the class only, and keeps what it says yes to.</summary>
        public static void Filter(FakeWorld world)
        {
            Fresh();
            var first = UnitAt(world, 10.0f, 10.0f);
            var second = UnitAt(world, 20.0f, 20.0f);
            var third = UnitAt(world, 30.0f, 30.0f);
            UnitAt(world, 900.0f, 900.0f);
            var asked = 0;
            var found = Find.InRange<Unit>(0.0f, 0.0f, 100.0f, u =>
            {
                asked++;
                return u != second;
            });
            Expect.Equal(3, asked, "asked of the three inside, not of the unit far away");
            Expect.Equal(2, found.Count, "two are kept");
            Expect.Same(first, found[0], "the first");
            Expect.Same(third, found[1], "and the third");
            var closest = Find.Closest<Unit>(0.0f, 0.0f, 100.0f, u => u != first);
            Expect.Same(second, closest, "the closest that the filter keeps");
            var rect = Find.InRect<Unit>(new Bounds(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 100.0f)), u => u == third);
            Expect.Equal(1, rect.Count, "a filter on a rect");
            var line = Find.AlongLine<Unit>(0.0f, 0.0f, 100.0f, 100.0f, 50.0f, u => u != third);
            Expect.Equal(2, line.Count, "a filter along a line");
            var polygon = new Polygon(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 0.0f), new Vector2(100.0f, 100.0f), new Vector2(0.0f, 100.0f));
            var inside = Find.InPolygon<Unit>(polygon, u => u != first);
            Expect.Equal(2, inside.Count, "a filter in a polygon");
        }

        /// <summary>A filling form clears the list first and returns the count, in every shape.</summary>
        public static void Fills(FakeWorld world)
        {
            Fresh();
            var old = UnitAt(world, 800.0f, 800.0f);
            var a = UnitAt(world, 10.0f, 10.0f);
            var b = UnitAt(world, 20.0f, 20.0f);
            var results = new List<Unit> { old, old, old, old };
            var count = Find.InRange(0.0f, 0.0f, 100.0f, results);
            Expect.Equal(2, count, "the count");
            Expect.Equal(2, results.Count, "the old results are gone");
            Expect.Same(a, results[0], "the first");
            Expect.Same(b, results[1], "the second");
            Expect.Equal(2, Find.InRange(new Vector2(0.0f, 0.0f), 100.0f, results), "the vector form");
            results.Add(old);
            var bounds = new Bounds(new Vector2(0.0f, 0.0f), new Vector2(15.0f, 15.0f));
            Expect.Equal(1, Find.InRect(bounds, results), "a rect");
            Expect.Equal(1, results.Count, "cleared before");
            results.Add(old);
            var polygon = new Polygon(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 0.0f), new Vector2(100.0f, 100.0f), new Vector2(0.0f, 100.0f));
            Expect.Equal(2, Find.InPolygon(polygon, results), "a polygon");
            Expect.Equal(2, results.Count, "cleared before");
            results.Add(old);
            Expect.Equal(2, Find.AlongLine(0.0f, 0.0f, 100.0f, 100.0f, 50.0f, results), "along a line");
            Expect.Equal(2, results.Count, "cleared before");
            results.Add(old);
            Expect.Equal(2, Find.AlongLine(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 100.0f), 50.0f, results), "along a line by vectors");
            results.Add(old);
            Expect.Equal(1, Find.Closest(0.0f, 0.0f, 1, 100.0f, results), "the closest one");
            Expect.Equal(1, results.Count, "cleared before");
            Expect.Same(a, results[0], "the nearest");
            results.Add(old);
            Expect.Equal(0, Find.InRange(-500.0f, -500.0f, 10.0f, results), "nothing found");
            Expect.Equal(0, results.Count, "an empty list");
        }

        /// <summary>A rect includes its edges and corners: a unit on the edge is in, one a step past it is out.</summary>
        public static void InRect(FakeWorld world)
        {
            Fresh();
            var corner = UnitAt(world, 200.0f, 100.0f);
            var edge = UnitAt(world, 200.0f, 50.0f);
            var middle = UnitAt(world, 100.0f, 50.0f);
            var low = UnitAt(world, 0.0f, 0.0f);
            var out1 = UnitAt(world, 201.0f, 50.0f);
            var out2 = UnitAt(world, 100.0f, -1.0f);
            var bounds = new Bounds(new Vector2(0.0f, 0.0f), new Vector2(200.0f, 100.0f));
            var found = Find.InRect<Unit>(bounds);
            Expect.Equal(4, found.Count, "four inside, edges included");
            Expect.True(Holds(found, corner) && Holds(found, edge) && Holds(found, middle) && Holds(found, low), "the corner, the edge, the middle and the low corner");
            Expect.True(!Holds(found, out1) && !Holds(found, out2), "one past the edge is out");
            var swapped = Find.InRect<Unit>(new Bounds(new Vector2(200.0f, 100.0f), new Vector2(0.0f, 0.0f)));
            Expect.Equal(4, swapped.Count, "bounds made from the other corners are the same");
        }

        /// <summary>A concave polygon: a U with a notch cut from the top. A unit in the notch is outside, units in the arms and the base inside, a unit on the notch's edge inside.</summary>
        public static void InPolygon(FakeWorld world)
        {
            Fresh();
            var polygon = new Polygon(
                new Vector2(0.0f, 0.0f),
                new Vector2(300.0f, 0.0f),
                new Vector2(300.0f, 300.0f),
                new Vector2(200.0f, 300.0f),
                new Vector2(200.0f, 100.0f),
                new Vector2(100.0f, 100.0f),
                new Vector2(100.0f, 300.0f),
                new Vector2(0.0f, 300.0f));
            var notch = UnitAt(world, 150.0f, 200.0f);
            var left = UnitAt(world, 50.0f, 200.0f);
            var right = UnitAt(world, 250.0f, 200.0f);
            var baseUnit = UnitAt(world, 150.0f, 50.0f);
            var edge = UnitAt(world, 100.0f, 200.0f);
            var outside = UnitAt(world, 350.0f, 200.0f);
            var found = Find.InPolygon<Unit>(polygon);
            Expect.Equal(4, found.Count, "the arms, the base and the notch's edge");
            Expect.True(Holds(found, left) && Holds(found, right) && Holds(found, baseUnit) && Holds(found, edge), "the four inside");
            Expect.True(!Holds(found, notch), "the notch is outside");
            Expect.True(!Holds(found, outside), "and so is what is beside the polygon");
        }

        /// <summary>Along a line of width 100: a unit 49 from the segment is in, 51 out, within 50 of the end is in, past it out, and the same before the start.</summary>
        public static void AlongLine(FakeWorld world)
        {
            Fresh();
            var inside = UnitAt(world, 250.0f, 49.0f);
            var below = UnitAt(world, 250.0f, -49.0f);
            var outsideUnit = UnitAt(world, 250.0f, 51.0f);
            var afterIn = UnitAt(world, 549.0f, 0.0f);
            var afterOut = UnitAt(world, 551.0f, 0.0f);
            var beforeIn = UnitAt(world, -49.0f, 0.0f);
            var beforeOut = UnitAt(world, -51.0f, 0.0f);
            var found = Find.AlongLine<Unit>(0.0f, 0.0f, 500.0f, 0.0f, 100.0f);
            Expect.Equal(4, found.Count, "four are within 50 of the segment");
            Expect.True(Holds(found, inside) && Holds(found, below), "49 from the segment, on either side, is in");
            Expect.True(Holds(found, afterIn) && Holds(found, beforeIn), "49 past an end is in");
            Expect.True(!Holds(found, outsideUnit), "51 from the segment is out");
            Expect.True(!Holds(found, afterOut) && !Holds(found, beforeOut), "51 past an end is out");
            var byVector = Find.AlongLine<Unit>(new Vector2(0.0f, 0.0f), new Vector2(500.0f, 0.0f), 100.0f);
            Expect.Equal(4, byVector.Count, "the vector form finds the same");
            var point = Find.AlongLine<Unit>(0.0f, 0.0f, 0.0f, 0.0f, 100.0f);
            Expect.Equal(1, point.Count, "a segment of no length is a circle of half the width: only the unit within 50 of the point");
            Expect.Same(beforeIn, point[0], "the unit 49 from it");
            Expect.Equal(0, Find.AlongLine<Unit>(0.0f, 0.0f, 500.0f, 0.0f, -1.0f).Count, "a width below 0 finds nothing");
        }

        /// <summary>A slanted line: the distance is to the segment itself, not to its bounding box.</summary>
        public static void AlongSlantedLine(FakeWorld world)
        {
            Fresh();
            var onLine = UnitAt(world, 200.0f, 200.0f);
            var nearLine = UnitAt(world, 230.0f, 170.0f);
            var farLine = UnitAt(world, 300.0f, 100.0f);
            var found = Find.AlongLine<Unit>(0.0f, 0.0f, 400.0f, 400.0f, 100.0f);
            Expect.Equal(2, found.Count, "the one on the line and the one 42 from it; the one 141 from it is in the bounding box and out");
            Expect.True(Holds(found, onLine) && Holds(found, nearLine), "the two");
            Expect.True(!Holds(found, farLine), "the far one is out");
        }

        /// <summary>The closest: the nearest within the range, null when none is, and the nearest is the first of those at the same distance.</summary>
        public static void Closest(FakeWorld world)
        {
            Fresh();
            var far = UnitAt(world, 400.0f, 0.0f);
            var near = UnitAt(world, 0.0f, 150.0f);
            var twin = UnitAt(world, 0.0f, -150.0f);
            var closest = Find.Closest<Unit>(0.0f, 0.0f, 500.0f);
            Expect.Same(twin, closest, "the nearest, and of two at 150 the one in the lower row, which the grid reads first");
            Expect.Same(twin, Find.Closest<Unit>(new Vector2(0.0f, 0.0f), 500.0f), "the vector form");
            Expect.Same(far, Find.Closest<Unit>(400.0f, 100.0f, 500.0f, u => u == far), "the filtered");
            Expect.True(Find.Closest<Unit>(0.0f, 0.0f, 100.0f) == null, "none within 100 gives null");
            Expect.Same(far, Find.Closest<Unit>(400.0f, 0.0f, 0.0f), "a range of 0 finds what is on the point");
            Expect.True(Find.Closest<Unit>(0.0f, 0.0f, -1.0f) == null, "a range below 0 gives null");
            Expect.True(near != null, "the other one at 150 exists");
        }

        /// <summary>The closest three of five, nearest first: ties in the grid's order, which is not the order the units were made in.</summary>
        public static void ClosestN(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 300.0f, 0.0f);
            var b = UnitAt(world, 0.0f, 100.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var d = UnitAt(world, 0.0f, 200.0f);
            var e = UnitAt(world, -200.0f, 0.0f);
            var results = new List<Unit>();
            Expect.Equal(3, Find.Closest(0.0f, 0.0f, 3, 1000.0f, results), "three of five");
            Expect.Same(b, results[0], "the nearest, at 100");
            Expect.Same(e, results[1], "of the three at 200, the one in the cell the grid reads first, left of the others");
            Expect.Same(c, results[2], "then the next in the grid's order");
            Expect.Equal(5, Find.Closest(0.0f, 0.0f, 10, 1000.0f, results), "asking for more than there are gives all five");
            Expect.Same(b, results[0], "100");
            Expect.Same(e, results[1], "200, the first in the grid");
            Expect.Same(c, results[2], "200");
            Expect.Same(d, results[3], "200, the last in the grid");
            Expect.Same(a, results[4], "300 last");
            Expect.Equal(4, Find.Closest(new Vector2(0.0f, 0.0f), 10, 250.0f, results), "the range cuts the farthest off, by a vector");
            Expect.Equal(0, Find.Closest(0.0f, 0.0f, 0, 1000.0f, results), "a count of 0 finds nothing");
            Expect.Equal(0, results.Count, "and clears the list");
            var kept = Find.Closest(0.0f, 0.0f, 2, 1000.0f, results, u => u != b);
            Expect.Equal(2, kept, "with a filter");
            Expect.Same(e, results[0], "without the nearest");
            Expect.Same(c, results[1], "the next two");
        }

        /// <summary>A unit that moved 60 since its cell was last read is still found at the edge of its range: the check looks in the cells within the slack, and reads the unit where it is now.</summary>
        public static void Stale(FakeWorld world)
        {
            Fresh();
            var unit = UnitAt(world, 255.0f, 0.0f);
            Tracking.NeedFor(typeof(Unit));
            var entry = Tracking.EntryOf(unit)!;
            Expect.Equal(4 * 8 + 4, entry.Cell, "tracked in the cell [0, 256)");
            world.MoveUnit(unit.Handle, 315.0f, 0.0f);
            Expect.Equal(4 * 8 + 4, entry.Cell, "the cell has not been read again");
            var found = Find.InRange<Unit>(615.0f, 0.0f, 300.0f);
            Expect.Equal(1, found.Count, "found at 300 from the point, where it is now");
            Expect.Same(unit, found[0], "the unit");
            Expect.Equal(4 * 8 + 5, entry.Cell, "and its cell follows the read");
            world.MoveUnit(unit.Handle, 375.0f, 0.0f);
            Expect.Equal(0, Find.InRange<Unit>(615.0f, 0.0f, 200.0f).Count, "a unit that moved out of the range is not found, whatever cell it was in");
            Expect.Equal(1, Find.InRange<Unit>(615.0f, 0.0f, 240.0f).Count, "but is found by a range that reaches its new place");
        }

        /// <summary>A dead, hidden or loaded unit, an item that is carried and a destructable that is dead have no place, and are not found; one that is shown again is.</summary>
        public static void NoPlace(FakeWorld world)
        {
            var fake = Fresh();
            var alive = UnitAt(world, 10.0f, 10.0f);
            var dead = UnitAt(world, 20.0f, 20.0f);
            var hidden = UnitAt(world, 30.0f, 30.0f);
            var loaded = UnitAt(world, 40.0f, 40.0f);
            var carried = ItemAt(world, 50.0f, 50.0f);
            var lying = ItemAt(world, 60.0f, 60.0f);
            alive.Give(carried);
            world.SlayUnit(dead.Handle);
            fake.Hidden.Add(hidden.Handle);
            fake.Loaded.Add(loaded.Handle);
            var found = Find.InRange<Actor>(0.0f, 0.0f, 200.0f);
            Expect.Equal(2, found.Count, "the living unit and the item on the ground");
            Expect.True(Holds(found, alive) && Holds(found, lying), "those two");
            fake.Hidden.Remove(hidden.Handle);
            Expect.Equal(3, Find.InRange<Actor>(0.0f, 0.0f, 200.0f).Count, "a unit shown again is found again");
            Expect.True(Find.Closest<Unit>(20.0f, 20.0f, 5.0f) == null, "and the dead one is not the closest");
        }

        /// <summary>Two runs over the same moves give the same list, in the same order, whatever the units' cells were doing between.</summary>
        public static void Order(FakeWorld world)
        {
            var first = Scene(world);
            var second = Scene(world);
            Expect.True(first.Count > 3, "the scene finds something");
            Expect.Equal(first.Count, second.Count, "the same number");
            for (var i = 0; i < first.Count; i++)
            {
                Expect.Equal(first[i], second[i], "the same unit at place " + i);
            }
        }

        /// <summary>Makes ten units, moves them over a few steps so that their cells change, finds them in a range, a rect and along a line, and gives the numbers of the units found in the order they came; the units are removed after.</summary>
        private static List<int> Scene(FakeWorld world)
        {
            Fresh();
            var units = new List<Unit>();
            for (var i = 0; i < 10; i++)
            {
                units.Add(UnitAt(world, i * 40.0f - 200.0f, (i * 7 % 10) * 30.0f));
            }

            Tracking.NeedFor(typeof(Unit));
            for (var step = 0; step < 6; step++)
            {
                for (var i = 0; i < 10; i++)
                {
                    world.MoveUnit(units[i].Handle, i * 40.0f - 200.0f + step * (i % 3) * 25.0f, (i * 7 % 10) * 30.0f - step * (i % 4) * 20.0f);
                }

                Steps(world, 3);
            }

            var numbers = new List<int>();
            var found = Find.InRange<Unit>(0.0f, 0.0f, 400.0f);
            AddNumbers(numbers, units, found);
            found = Find.InRect<Unit>(new Bounds(new Vector2(-100.0f, -100.0f), new Vector2(300.0f, 200.0f)));
            AddNumbers(numbers, units, found);
            found = Find.AlongLine<Unit>(-200.0f, 0.0f, 300.0f, 100.0f, 150.0f);
            AddNumbers(numbers, units, found);
            for (var i = 0; i < units.Count; i++)
            {
                units[i].Remove();
            }

            return numbers;
        }

        /// <summary>Adds the number each found unit has in the list the scene made, and -1 for a separator.</summary>
        private static void AddNumbers(List<int> numbers, List<Unit> units, List<Unit> found)
        {
            for (var i = 0; i < found.Count; i++)
            {
                numbers.Add(units.IndexOf(found[i]));
            }

            numbers.Add(-1);
        }

        /// <summary>A Find from a gizmo's OnTick, during the cycle, works: it finds what is there, and the cycle goes on with the gizmos after it and the cell checks.</summary>
        public static void InsideReaction(FakeWorld world)
        {
            Fresh();
            var near = UnitAt(world, 10.0f, 10.0f);
            var moving = UnitAt(world, 500.0f, 500.0f);
            Tracking.NeedFor(typeof(Unit));
            var found = new List<Unit>();
            var count = -1;
            var first = new Well(Interactions.Step);
            first.Ticking = () =>
            {
                count = Find.InRange(0.0f, 0.0f, 100.0f, found);
            };
            var second = new Well(Interactions.Step);
            Gizmo.Create(first, 0.0f, 0.0f);
            Gizmo.Create(second, 0.0f, 0.0f);
            Steps(world, 1);
            Expect.Equal(1, count, "the Find from inside OnTick found the unit");
            Expect.Same(near, found[0], "the unit");
            Expect.Equal(1, first.TickSteps.Count, "the gizmo ticked once");
            Expect.Equal(1, second.TickSteps.Count, "and the gizmo after it as well");
            world.MoveUnit(moving.Handle, 20.0f, 20.0f);
            Steps(world, 4);
            Expect.Equal(2, count, "four steps later the cell check has read the other unit, and the Find sees it too");
            Expect.Equal(5, first.TickSteps.Count, "the gizmo kept ticking");
            Expect.Equal(5, second.TickSteps.Count, "and the other as well");
        }

        /// <summary>A Find from inside the filter of another Find does not change the candidates the outer one is walking: the outer result is the same as without the inner one, and the inner finds its own.</summary>
        public static void Nested(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 50.0f, 0.0f);
            var c = UnitAt(world, 100.0f, 0.0f);
            UnitAt(world, 700.0f, 0.0f);
            UnitAt(world, 750.0f, 0.0f);
            var plain = Find.InRange<Unit>(0.0f, 0.0f, 1000.0f);
            Expect.Equal(5, plain.Count, "all five without a filter");
            var inner = new List<int>();
            var held = 0;
            var nested = Find.InRange<Unit>(0.0f, 0.0f, 1000.0f, u =>
            {
                held = Find.Pending;
                var near = Find.InRange<Unit>(u.X, u.Y, 120.0f);
                inner.Add(near.Count);
                return near.Count >= 2;
            });
            Expect.True(held > 0, "the outer search holds its candidates while the filter runs");
            Expect.Equal(0, Find.Pending, "and no search holds any when they are done");
            Expect.Equal(5, inner.Count, "the filter ran for every unit, each time with a nested Find");
            Expect.Equal(3, inner[0], "the unit at 0 has three within 120");
            Expect.Equal(3, inner[1], "so has the one at 50");
            Expect.Equal(3, inner[2], "and the one at 100");
            Expect.Equal(2, inner[3], "the one at 700 has two");
            Expect.Equal(2, inner[4], "and the one at 750");
            Expect.Equal(5, nested.Count, "all five kept");
            for (var i = 0; i < plain.Count; i++)
            {
                Expect.Same(plain[i], nested[i], "the same order at place " + i);
            }

            var closest = Find.Closest<Unit>(0.0f, 0.0f, 1000.0f, u => Find.Closest<Unit>(u.X, u.Y, 60.0f, w => w != u) != null);
            Expect.Same(a, closest, "a Closest with a Find inside it");
            var results = new List<Unit>();
            var count = Find.Closest(0.0f, 0.0f, 3, 1000.0f, results, u => Find.Closest(u.X, u.Y, 2, 1000.0f, new List<Unit>(), w => w != u) == 2);
            Expect.Equal(3, count, "a Closest of several with another inside it");
            Expect.Same(a, results[0], "the nearest");
            Expect.Same(b, results[1], "the next");
            Expect.Same(c, results[2], "and the third");
            Expect.Equal(0, Find.Pending, "no candidate or distance is left behind");
        }

        /// <summary>A Find whose filter throws leaves the candidates as they were: the next Find is right.</summary>
        public static void Throwing(FakeWorld world)
        {
            Fresh();
            UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 10.0f, 0.0f);
            Expect.Throws<System.InvalidOperationException>(() => Find.InRange<Unit>(0.0f, 0.0f, 100.0f, u =>
            {
                throw new System.InvalidOperationException("boom");
            }), "the filter throws");
            Expect.Equal(0, Find.Pending, "no candidate is left behind");
            Expect.Equal(2, Find.InRange<Unit>(0.0f, 0.0f, 100.0f).Count, "and the next Find is right");
        }

        /// <summary>A Closest of several whose filter throws after it has kept a result leaves neither the candidates nor the distances behind: the next one is right.</summary>
        public static void ThrowingClosest(FakeWorld world)
        {
            Fresh();
            var near = UnitAt(world, 10.0f, 0.0f);
            var middle = UnitAt(world, 20.0f, 0.0f);
            UnitAt(world, 30.0f, 0.0f);
            var calls = 0;
            var results = new List<Unit>();
            Expect.Throws<System.InvalidOperationException>(() => Find.Closest(0.0f, 0.0f, 3, 100.0f, results, u =>
            {
                calls++;
                if (calls == 3)
                {
                    throw new System.InvalidOperationException("boom");
                }

                return true;
            }), "the filter throws on the third unit, with two kept");
            Expect.Equal(2, calls - 1, "two had been kept");
            Expect.Equal(0, Find.Pending, "no candidate and no distance is left behind");
            Expect.Equal(2, Find.Closest(0.0f, 0.0f, 2, 100.0f, results), "the next Closest is right");
            Expect.Same(near, results[0], "the nearest");
            Expect.Same(middle, results[1], "the next");
        }

        /// <summary>Things beyond the world's edge are in the edge cells and are found: a unit at x = 3000, one at x = -5000 and one exactly on the edge, by a range, a rect and a closest, with boxes wholly outside the world and boxes that straddle its edge.</summary>
        public static void Edge(FakeWorld world)
        {
            Fresh();
            var inside = UnitAt(world, 0.0f, 0.0f);
            var east = UnitAt(world, 3000.0f, 0.0f);
            var west = UnitAt(world, -5000.0f, 0.0f);
            var onEdge = UnitAt(world, 1024.0f, 0.0f);
            Tracking.NeedFor(typeof(Unit));
            Expect.Equal(4 * 8 + 7, Tracked(east).Cell, "the unit past the east edge is in the edge cell");
            Expect.Equal(4 * 8 + 0, Tracked(west).Cell, "the unit past the west edge is in the west edge cell");
            var found = Find.InRange<Unit>(3000.0f, 0.0f, 100.0f);
            Expect.Equal(1, found.Count, "a range wholly outside the world finds the unit there");
            Expect.Same(east, found[0], "the east one");
            found = Find.InRange<Unit>(-5000.0f, 0.0f, 100.0f);
            Expect.Equal(1, found.Count, "and on the other side");
            Expect.Same(west, found[0], "the west one");
            found = Find.InRange<Unit>(1024.0f, 0.0f, 10.0f);
            Expect.Equal(1, found.Count, "a range on the edge finds the unit on it only");
            Expect.Same(onEdge, found[0], "the one on the edge");
            Expect.Equal(0, Find.InRange<Unit>(5000.0f, 5000.0f, 100.0f).Count, "a range outside the world with nothing in it finds nothing");
            found = Find.InRect<Unit>(new Bounds(new Vector2(2900.0f, -50.0f), new Vector2(3100.0f, 50.0f)));
            Expect.Equal(1, found.Count, "a rect wholly outside the world");
            Expect.Same(east, found[0], "finds the east one");
            found = Find.InRect<Unit>(new Bounds(new Vector2(-6000.0f, -50.0f), new Vector2(-4000.0f, 50.0f)));
            Expect.Equal(1, found.Count, "a rect past the west edge");
            Expect.Same(west, found[0], "finds the west one");
            found = Find.InRect<Unit>(new Bounds(new Vector2(900.0f, -50.0f), new Vector2(3100.0f, 50.0f)));
            Expect.Equal(2, found.Count, "a rect that straddles the east edge");
            Expect.True(Holds(found, onEdge) && Holds(found, east), "finds the one on the edge and the one past it");
            found = Find.InRect<Unit>(new Bounds(new Vector2(-6000.0f, -50.0f), new Vector2(3100.0f, 50.0f)));
            Expect.Equal(4, found.Count, "a rect that covers the world and more finds all four");
            Expect.Same(east, Find.Closest<Unit>(2500.0f, 0.0f, 600.0f), "the closest to a point outside the world");
            Expect.Same(west, Find.Closest<Unit>(-4000.0f, 0.0f, 2000.0f), "and on the other side");
            Expect.Same(onEdge, Find.Closest<Unit>(1100.0f, 0.0f, 2000.0f), "the one on the edge, from a point just past it");
            Expect.True(Find.Closest<Unit>(2500.0f, 0.0f, 400.0f) == null, "none within a range that does not reach");
            var results = new List<Unit>();
            Expect.Equal(4, Find.Closest(0.0f, 0.0f, 10, 10000.0f, results), "all four by distance");
            Expect.Same(inside, results[0], "0");
            Expect.Same(onEdge, results[1], "1024");
            Expect.Same(east, results[2], "3000");
            Expect.Same(west, results[3], "5000");
        }

        /// <summary>The entry of a thing the case knows is tracked.</summary>
        private static Entry Tracked(Actor thing)
        {
            var entry = Tracking.EntryOf(thing);
            Expect.True(entry != null, "the thing is tracked");
            return entry!;
        }

        /// <summary>Every cell's entries in order, each cell followed by a null.</summary>
        private static List<Entry?> CellLists()
        {
            var all = new List<Entry?>();
            var grid = Interactions.Grid;
            for (var i = 0; i < 64; i++)
            {
                var cell = grid.CellList(i);
                if (cell != null)
                {
                    for (var j = 0; j < cell.Count; j++)
                    {
                        all.Add(cell[j]);
                    }
                }

                all.Add(null);
            }

            return all;
        }

        /// <summary>What an entry keeps, as numbers: X, Y, the step it was read at and its cell.</summary>
        private static List<float> Kept(List<Entry> entries)
        {
            var kept = new List<float>();
            for (var i = 0; i < entries.Count; i++)
            {
                kept.Add(entries[i].X);
                kept.Add(entries[i].Y);
                kept.Add(entries[i].ReadStep);
                kept.Add(entries[i].Cell);
            }

            return kept;
        }

        /// <summary>Inside Local.For a Find of a tracked kind reads where things are now and changes nothing: no entry's place, step or cell, and no cell list, is touched, though a unit has moved across a cell.</summary>
        public static void LocalTracked(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 10.0f, 10.0f);
            var b = UnitAt(world, 255.0f, 10.0f);
            var well = Gizmo.Create(new Well(), 320.0f, 20.0f);
            Expect.Equal(1, Find.InRange<Actor>(0.0f, 0.0f, 100.0f).Count, "outside Local.For the first Find tracks the units and finds the one near");
            var entries = new List<Entry>();
            entries.Add(Tracked(a));
            entries.Add(Tracked(b));
            entries.Add(Tracked(well));
            var kept = Kept(entries);
            var lists = CellLists();
            world.MoveUnit(a.Handle, 50.0f, 10.0f);
            world.MoveUnit(b.Handle, 315.0f, 10.0f);
            var found = new List<Actor>();
            var closest = new List<Unit>();
            Unit? nearest = null;
            Local.For(Player.Get(0), () =>
            {
                found = Find.InRange<Actor>(400.0f, 10.0f, 100.0f);
                Find.Closest(400.0f, 10.0f, 2, 2000.0f, closest);
                nearest = Find.Closest<Unit>(50.0f, 10.0f, 10.0f);
            });
            Expect.Equal(2, found.Count, "the unit that crossed into the next cell, and the gizmo");
            Expect.True(Holds(found, b) && Holds(found, well), "found where they are now");
            Expect.Equal(2, closest.Count, "a closest of several");
            Expect.Same(b, closest[0], "the unit at 315 is nearer to the point than the one at 50");
            Expect.Same(a, closest[1], "then the other");
            Expect.Same(a, nearest, "a unit that moved 40 is found where it is now: its entry still says 10");
            var after = Kept(entries);
            Expect.Equal(kept.Count, after.Count, "the same entries");
            for (var i = 0; i < kept.Count; i++)
            {
                Expect.Near(kept[i], after[i], "an entry's place, step and cell are as they were, number " + i);
            }

            var listsAfter = CellLists();
            Expect.Equal(lists.Count, listsAfter.Count, "the cell lists are as long");
            for (var i = 0; i < lists.Count; i++)
            {
                Expect.Same(lists[i], listsAfter[i], "and in the same order, place " + i);
            }

            Expect.Equal(2, Find.InRange<Actor>(400.0f, 10.0f, 100.0f).Count, "outside Local.For the same Find reads and moves the entry");
            Expect.Equal(4 * 8 + 5, Tracked(b).Cell, "to the cell of its place");
        }

        /// <summary>Inside Local.For, in a step, a place read this step is the one kept, and one not read this step is read from the game; outside a step it is always read.</summary>
        public static void LocalCached(FakeWorld world)
        {
            Fresh();
            var unit = UnitAt(world, 10.0f, 10.0f);
            Tracking.NeedFor(typeof(Unit));
            var entry = Tracked(unit);
            var well = new Well(Interactions.Step);
            var cached = -1;
            well.Ticking = () =>
            {
                Find.InRange<Unit>(0.0f, 0.0f, 100.0f);
                world.MoveUnit(unit.Handle, 500.0f, 500.0f);
                Local.For(Player.Get(0), () => { cached = Find.InRange<Unit>(0.0f, 0.0f, 100.0f).Count; });
                well.Ticking = null;
            };
            Gizmo.Create(well, 0.0f, 0.0f);
            Steps(world, 1);
            Expect.Equal(1, cached, "inside the step the place read this step is used, though the unit has moved");
            Expect.Near(10.0f, entry.X, "and it is kept as read");
            var live = -1;
            Local.For(Player.Get(0), () => { live = Find.InRange<Unit>(0.0f, 0.0f, 100.0f).Count; });
            Expect.Equal(0, live, "outside a step the place is read from the game");
            Expect.Near(10.0f, entry.X, "and still not kept");
        }

        /// <summary>Inside Local.For a Find of a kind nobody asked for finds the live things of the game in the runtime's order, and tracks nothing: the kind stays untracked and the cycle's timer stays stopped.</summary>
        public static void LocalUntracked(FakeWorld world)
        {
            Fresh();
            var first = UnitAt(world, 10.0f, 10.0f);
            UnitAt(world, 900.0f, 900.0f);
            var second = UnitAt(world, 20.0f, 20.0f);
            var item = ItemAt(world, 30.0f, 30.0f);
            var found = new List<Actor>();
            var units = new List<Unit>();
            var polygon = new Polygon(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 0.0f), new Vector2(100.0f, 100.0f), new Vector2(0.0f, 100.0f));
            var inPolygon = new List<Unit>();
            Local.For(Player.Get(0), () =>
            {
                found = Find.InRange<Actor>(0.0f, 0.0f, 100.0f);
                units = Find.InRect<Unit>(new Bounds(new Vector2(0.0f, 0.0f), new Vector2(100.0f, 100.0f)));
                inPolygon = Find.InPolygon<Unit>(polygon);
            });
            Expect.Equal(3, found.Count, "the two units and the item");
            Expect.Same(first, found[0], "the units in the order the runtime took them in");
            Expect.Same(second, found[1], "the next");
            Expect.Same(item, found[2], "then the item");
            Expect.Equal(2, units.Count, "a rect");
            Expect.Equal(2, inPolygon.Count, "a polygon");
            Expect.True(Tracking.EntryOf(first) == null, "the unit is not tracked");
            Expect.True(Tracking.EntryOf(item) == null, "nor the item");
            Expect.Equal(0, Tracking.Count, "nothing is tracked");
            Expect.True(!Tracking.Running, "and the cycle's timer has not started");
            Expect.Equal(0, Find.Pending, "no candidates are left");
            Expect.Equal(2, Find.InRange<Unit>(0.0f, 0.0f, 100.0f).Count, "outside Local.For the same Find tracks the kind");
            Expect.True(Tracking.EntryOf(first) != null, "now the unit is tracked");
            Expect.True(Tracking.Running, "and the timer runs");
        }

        /// <summary>Inside Local.For a Find of Actor looks in the grid for what is tracked, here a gizmo, and in the runtime's lists for the kinds that are not, here units, and tracks no unit.</summary>
        public static void LocalMixed(FakeWorld world)
        {
            Fresh();
            var well = Gizmo.Create(new Well(), 15.0f, 15.0f);
            var unit = UnitAt(world, 10.0f, 10.0f);
            var found = new List<Actor>();
            Local.For(Player.Get(0), () => { found = Find.InRange<Actor>(0.0f, 0.0f, 100.0f); });
            Expect.Equal(2, found.Count, "the gizmo and the unit");
            Expect.True(Holds(found, well) && Holds(found, unit), "both");
            Expect.True(Tracking.EntryOf(unit) == null, "the unit stays untracked");
            Expect.Equal(1, Tracking.Count, "only the gizmo is tracked");
        }
    }
}
