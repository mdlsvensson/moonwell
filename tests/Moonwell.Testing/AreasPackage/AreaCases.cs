using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for areas: units entering, staying in and leaving, on the fake game.</summary>
    public static class AreaCases
    {
        /// <summary>Installs the package on a fake world with no World Editor regions and returns it.</summary>
        private static FakeAreaWorld Fresh()
        {
            var fake = new FakeAreaWorld();
            AreaSystem.Install(fake, new string[0], new float[0]);
            return fake;
        }

        /// <summary>A rect from (0, 0) to (256, 256).</summary>
        private static AreaShape Square()
        {
            return AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(256f, 256f)));
        }

        /// <summary>Makes a unit of the game at a place and takes it in; a hero when asked.</summary>
        private static Unit UnitAt(FakeWorld world, float x, float y, bool hero = false)
        {
            var handle = world.NewUnit(ObjectTypes.FourCC(hero ? "Hpal" : "hfoo"), 0, hero);
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Adds a labelled area to the game and lets its first, 0 second timer run.</summary>
        private static LogArea Added(FakeWorld world, LogArea area)
        {
            Game.Instance.Add(area);
            world.Advance(0f);
            return area;
        }

        /// <summary>Fails unless the area's log is exactly these lines.</summary>
        private static void ExpectLog(LogArea area, string[] expected, string what)
        {
            var actual = string.Join(", ", area.Log.ToArray());
            Expect.Equal(string.Join(", ", expected), actual, what);
        }

        /// <summary>A unit enters, stays three ticks of a second and leaves: the reactions run in that order, and Since is 3 at the last tick.</summary>
        public static void EnterStayLeave(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var unit = UnitAt(world, 10f, 10f);
            area.Labels[unit] = "a";
            Expect.True(!area.Has(unit), "not inside yet");
            Expect.Near(0f, area.Since(unit), "Since is 0 while not inside");
            fake.Enter(fake.Regions[0], unit.Handle);
            Expect.True(area.Has(unit), "inside after the enter event");
            world.Advance(1f);
            world.Advance(1f);
            world.Advance(1f);
            fake.Leave(fake.Regions[0], unit.Handle);
            ExpectLog(area, new[] { "entered a", "stays a", "stays a", "stays a", "left a" }, "the reactions in order");
            Expect.Near(1f, area.SinceAtStay[0], "Since at the first tick");
            Expect.Near(3f, area.SinceAtStay[2], "Since at the last tick");
            Expect.True(!area.Has(unit), "outside after the leave event");
            Expect.Near(0f, area.Since(unit), "Since is 0 again");
            world.Advance(2f);
            Expect.Equal(5, area.Log.Count, "nothing runs after the unit left");
        }

        /// <summary>Accepts decides: a footman is refused, a hero accepted, and the footman enters at its next enter event once the area accepts it.</summary>
        public static void Accepts(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            area.Accept = u => u is Hero;
            var footman = UnitAt(world, 10f, 10f);
            var hero = UnitAt(world, 20f, 20f, true);
            area.Labels[footman] = "f";
            area.Labels[hero] = "h";
            fake.Enter(fake.Regions[0], footman.Handle);
            fake.Enter(fake.Regions[0], hero.Handle);
            ExpectLog(area, new[] { "entered h" }, "only the hero entered");
            Expect.True(!area.Has(footman), "the footman is not inside");
            area.Accept = null;
            world.Advance(1f);
            Expect.True(!area.Has(footman), "nothing asks again by itself");
            fake.Enter(fake.Regions[0], footman.Handle);
            ExpectLog(area, new[] { "entered h", "entered f" }, "the footman at its next enter event");
        }

        /// <summary>Units inside when the area is added enter after its 0 second timer, in Moonwell's order of units and not the game's.</summary>
        public static void AlreadyInside(FakeWorld world)
        {
            var fake = Fresh();
            var first = UnitAt(world, 10f, 10f);
            var second = UnitAt(world, 20f, 20f);
            var outside = UnitAt(world, 900f, 900f);
            fake.Stand(second.Handle, 20f, 20f);
            fake.Stand(first.Handle, 10f, 10f);
            fake.Stand(outside.Handle, 900f, 900f);
            var area = new LogArea(Square());
            area.Labels[first] = "first";
            area.Labels[second] = "second";
            Game.Instance.Add(area);
            fake.Regions[0].Inside.Add(second.Handle);
            fake.Regions[0].Inside.Add(first.Handle);
            Expect.Equal(0, area.Log.Count, "nothing enters before the 0 second timer");
            world.Advance(0f);
            ExpectLog(area, new[] { "entered first", "entered second" }, "in the order of KnownUnits.Live");
            Expect.Equal(2, area.Units.Count, "two units inside");
            Expect.Same(first, area.Units[0], "the first entered is first");
        }

        /// <summary>A unit the sweep and an enter event both find enters once, whichever comes first.</summary>
        public static void SweepAndEvent(FakeWorld world)
        {
            var fake = Fresh();
            var early = UnitAt(world, 10f, 10f);
            var late = UnitAt(world, 20f, 20f);
            fake.Stand(early.Handle, 10f, 10f);
            fake.Stand(late.Handle, 20f, 20f);
            var area = new LogArea(Square());
            area.Labels[early] = "early";
            area.Labels[late] = "late";
            Game.Instance.Add(area);
            var made = fake.Regions[0];
            made.Inside.Add(early.Handle);
            made.Inside.Add(late.Handle);
            fake.Enter(made, early.Handle);
            world.Advance(0f);
            fake.Enter(made, late.Handle);
            ExpectLog(area, new[] { "entered early", "entered late" }, "each once");
        }

        /// <summary>A unit that dies inside leaves; a hero revived inside enters again, and one that is not a hero is not entered again by anything.</summary>
        public static void Death(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var hero = UnitAt(world, 10f, 10f, true);
            var footman = UnitAt(world, 20f, 20f);
            area.Labels[hero] = "h";
            area.Labels[footman] = "f";
            fake.Enter(fake.Regions[0], hero.Handle);
            fake.Enter(fake.Regions[0], footman.Handle);
            fake.NoPlace.Add(hero.Handle);
            fake.NoPlace.Add(footman.Handle);
            fake.Die(hero.Handle);
            fake.Die(footman.Handle);
            ExpectLog(area, new[] { "entered h", "entered f", "left h", "left f" }, "both left");
            world.Advance(1f);
            Expect.Equal(4, area.Log.Count, "a corpse does not come back by itself");
            fake.NoPlace.Remove(hero.Handle);
            fake.Revive(hero.Handle);
            ExpectLog(area, new[] { "entered h", "entered f", "left h", "left f", "entered h" }, "the hero entered again");
            Expect.True(area.Has(hero), "the hero is inside");
            fake.Revive(footman.Handle);
            Expect.Equal(5, area.Log.Count, "a revive of a unit that is still dead does nothing");
            fake.Leave(fake.Regions[0], hero.Handle);
            fake.Die(hero.Handle);
            fake.Revive(hero.Handle);
            Expect.Equal(6, area.Log.Count, "left, died and revived outside: no enter");
            Expect.True(!area.Has(hero), "the hero stays outside");
        }

        /// <summary>A unit removed from the game while inside leaves once, through the ending hook.</summary>
        public static void Removed(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var unit = UnitAt(world, 10f, 10f);
            area.Labels[unit] = "a";
            fake.Enter(fake.Regions[0], unit.Handle);
            unit.Remove();
            ExpectLog(area, new[] { "entered a", "left a" }, "left once");
            world.Advance(2f);
            Expect.Equal(2, area.Log.Count, "no stay after");
            Expect.Equal(0, area.Units.Count, "nothing inside");
        }

        /// <summary>Ending an area with two units inside lets both leave in the order they entered, destroys the region, and nothing later reaches a reaction.</summary>
        public static void AreaEnds(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var a = UnitAt(world, 10f, 10f);
            var b = UnitAt(world, 20f, 20f);
            var c = UnitAt(world, 30f, 30f);
            area.Labels[a] = "a";
            area.Labels[b] = "b";
            area.Labels[c] = "c";
            var made = fake.Regions[0];
            fake.Enter(made, b.Handle);
            fake.Enter(made, a.Handle);
            area.End();
            ExpectLog(area, new[] { "entered b", "entered a", "left b", "left a" }, "both left in entering order");
            Expect.True(made.Destroyed, "the region is destroyed");
            Expect.True(area.HasEnded, "the area ended");
            fake.Enter(made, c.Handle);
            fake.Leave(made, a.Handle);
            fake.Die(b.Handle);
            fake.Revive(b.Handle);
            world.Advance(3f);
            Expect.Equal(4, area.Log.Count, "later events reach no reaction");
            Expect.True(!AreaTracker.Running, "the package timer is not running");
        }

        /// <summary>Ending the area from inside OnUnitEntered lets that unit leave once, and nothing runs after.</summary>
        public static void EndInsideEnter(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var unit = UnitAt(world, 10f, 10f);
            area.Labels[unit] = "a";
            area.WhenEntered = u => area.End();
            fake.Enter(fake.Regions[0], unit.Handle);
            ExpectLog(area, new[] { "entered a", "left a" }, "entered and left once");
            world.Advance(3f);
            Expect.Equal(2, area.Log.Count, "nothing runs after");
            Expect.True(area.HasEnded, "the area ended");
        }

        /// <summary>A unit removed from inside OnUnitLeft does not leave a second time.</summary>
        public static void RemoveInsideLeft(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = UnitAt(world, 10f, 10f);
            area.Labels[unit] = "a";
            area.WhenLeft = u => u.Remove();
            fake.Enter(fake.Regions[0], unit.Handle);
            fake.Leave(fake.Regions[0], unit.Handle);
            ExpectLog(area, new[] { "entered a", "left a" }, "left once");
            Expect.True(unit.HasEnded, "the unit was removed");
        }

        /// <summary>A reaction that throws is reported under the area's class name and the reaction's, and the unit stays inside.</summary>
        public static void Throwing(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var unit = UnitAt(world, 10f, 10f);
            area.WhenEntered = u => throw new InvalidOperationException("boom");
            fake.Enter(fake.Regions[0], unit.Handle);
            Expect.Equal(1, world.Logged.Count, "one line logged");
            Expect.Equal("Moonwell: LogArea.OnUnitEntered failed: boom", world.Logged[0], "the logged line");
            Expect.True(area.Has(unit), "the unit is still inside");
            area.WhenLeft = u => throw new InvalidOperationException("bang");
            fake.Leave(fake.Regions[0], unit.Handle);
            Expect.Equal("Moonwell: LogArea.OnUnitLeft failed: bang", world.Logged[1], "a throwing OnUnitLeft");
            Expect.True(!area.Has(unit), "and the unit is out");
            area.Accept = u => throw new InvalidOperationException("no");
            fake.Enter(fake.Regions[0], unit.Handle);
            Expect.Equal("Moonwell: LogArea.Accepts failed: no", world.Logged[2], "a throwing Accepts");
            Expect.True(!area.Has(unit), "refuses the unit");
        }

        /// <summary>Every reaction runs with the area as the current owner.</summary>
        public static void ReactionOwner(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square(), 1f));
            var unit = UnitAt(world, 10f, 10f);
            fake.Enter(fake.Regions[0], unit.Handle);
            Expect.Same(area, area.OwnerSeen, "OnUnitEntered");
            area.OwnerSeen = null;
            world.Advance(1f);
            Expect.Same(area, area.OwnerSeen, "OnUnitStays");
            area.OwnerSeen = null;
            fake.Leave(fake.Regions[0], unit.Handle);
            Expect.Same(area, area.OwnerSeen, "OnUnitLeft");
        }

        /// <summary>A circle area makes the cells of the shape and no rect; a region area makes one rect and no cells; a union makes both; each region is listened to once, and the deaths once for the package.</summary>
        public static void Cells(FakeWorld world)
        {
            var fake = Fresh();
            var circle = AreaShape.Circle(new Vector2(0f, 0f), 48f);
            Game.Instance.Add(new LogArea(circle));
            var wanted = new List<float>();
            AreaCells.Of(circle, wanted);
            var made = fake.Regions[0];
            Expect.Equal(0, made.Rects.Count, "a circle adds no rect");
            Expect.Equal(8, made.Cells.Count, "a circle of radius 48 is 4 cells");
            for (var i = 0; i < wanted.Count; i++)
            {
                Expect.Near(wanted[i], made.Cells[i], "the cells are the Task 3 cells");
            }

            Expect.True(made.Listening, "the region is listened to");
            AreaSystem.Install(fake, new[] { "Shop" }, new[] { 0f, 0f, 256f, 128f });
            Game.Instance.Add(new LogArea(AreaShape.Region("Shop")));
            var shop = fake.Regions[1];
            Expect.Equal(4, shop.Rects.Count, "a region is one rect");
            Expect.Near(256f, shop.Rects[2], "its right edge");
            Expect.Near(128f, shop.Rects[3], "its top edge");
            Expect.Equal(0, shop.Cells.Count, "a region adds no cells");
            Game.Instance.Add(new LogArea(AreaShape.Of(AreaShape.Rect(new Bounds(new Vector2(0f, 0f), new Vector2(64f, 64f))), circle)));
            Expect.Equal(4, fake.Regions[2].Rects.Count, "a union holds its rect");
            Expect.Equal(8, fake.Regions[2].Cells.Count, "and its circle's cells");
            Expect.True(shop.Listening && fake.Regions[2].Listening, "each region is listened to");
            Expect.Equal(1, fake.DeathListens, "the deaths are listened to once");
        }

        /// <summary>An area added without a Shape is reported as an error from OnAdded and makes no region.</summary>
        public static void NoShape(FakeWorld world)
        {
            var fake = Fresh();
            Game.Instance.Add(new ShapelessArea());
            Expect.Equal(1, world.Logged.Count, "one line logged");
            Expect.Equal("Moonwell: ShapelessArea.OnAdded failed: An area needs a Shape: set it in its constructor.", world.Logged[0], "the logged line");
            Expect.Equal(0, fake.Regions.Count, "no region was made");
            Expect.True(Game.Instance.Get<ShapelessArea>() != null, "the component stays on the game");
            Game.Instance.Get<ShapelessArea>()!.End();
            Expect.Equal(1, world.Logged.Count, "and ends without a further error");
        }

        /// <summary>The shape is fixed once the area is added.</summary>
        public static void FixedOnceAdded(FakeWorld world)
        {
            Fresh();
            var area = Added(world, new LogArea(Square()));
            Expect.Throws<InvalidOperationException>(() => area.SetShape(Square()), "Shape after adding");
        }

        /// <summary>Reads: Units in entering order, Has, Contains, Bounds.</summary>
        public static void Reads(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var a = UnitAt(world, 10f, 10f);
            var b = UnitAt(world, 20f, 20f);
            fake.Enter(fake.Regions[0], b.Handle);
            fake.Enter(fake.Regions[0], a.Handle);
            Expect.Same(b, area.Units[0], "the first to enter is first");
            Expect.Same(a, area.Units[1], "the second to enter is second");
            Expect.True(area.Has(a) && area.Has(b), "both are inside");
            Expect.True(area.Contains(100f, 100f) && area.Contains(new Vector2(256f, 256f)), "Contains takes the shape's test");
            Expect.True(!area.Contains(257f, 100f), "and is false outside");
            Expect.True(area.Bounds == new Bounds(new Vector2(0f, 0f), new Vector2(256f, 256f)), "Bounds are the shape's");
            world.Advance(2f);
            Expect.Near(2f, area.Since(a), "Since counts game seconds");
        }

        /// <summary>The package has one timer: none while no area has a unit, one when any has, the first area's OnUnitStays twice for the second's once, a Tick of 0.04 every 2 steps, and a Tick of 0 never.</summary>
        public static void SharedTimer(FakeWorld world)
        {
            var fake = Fresh();
            var half = Added(world, new LogArea(Square(), 0.5f));
            var one = Added(world, new LogArea(Square(), 1f));
            var fast = Added(world, new LogArea(Square(), 0.04f));
            var none = Added(world, new LogArea(Square(), 0f));
            var a = UnitAt(world, 10f, 10f);
            var b = UnitAt(world, 20f, 20f);
            var c = UnitAt(world, 30f, 30f);
            var d = UnitAt(world, 40f, 40f);
            Expect.True(!AreaTracker.Running, "no timer while no area has a unit");
            var owned = Game.Instance.OwnedCount;
            fake.Enter(fake.Regions[0], a.Handle);
            Expect.True(AreaTracker.Running, "a timer once an area has a unit");
            Expect.Equal(owned + 1, Game.Instance.OwnedCount, "one timer more");
            fake.Enter(fake.Regions[1], b.Handle);
            fake.Enter(fake.Regions[2], c.Handle);
            fake.Enter(fake.Regions[3], d.Handle);
            Expect.Equal(owned + 1, Game.Instance.OwnedCount, "still one timer for four areas");
            world.Advance(2f);
            Expect.Equal(4, half.SinceAtStay.Count, "Tick 0.5 over 2 seconds");
            Expect.Equal(2, one.SinceAtStay.Count, "Tick 1 over 2 seconds");
            Expect.Equal(32, fast.SinceAtStay.Count, "Tick 0.04 is 2 steps: 32 in 2 seconds");
            Expect.Near(0.0625f, fast.SinceAtStay[0], "its first stay after 2 steps");
            Expect.Equal(0, none.SinceAtStay.Count, "Tick 0 never stays");
            fake.Leave(fake.Regions[0], a.Handle);
            fake.Leave(fake.Regions[1], b.Handle);
            fake.Leave(fake.Regions[2], c.Handle);
            Expect.True(AreaTracker.Running, "the fourth area still has a unit");
            fake.Leave(fake.Regions[3], d.Handle);
            Expect.True(!AreaTracker.Running, "the timer ends when no area has a unit");
            Expect.Equal(owned, Game.Instance.OwnedCount, "and is not owned any more");
            world.Advance(2f);
            Expect.Equal(4, half.SinceAtStay.Count, "nothing ran after");
        }

        /// <summary>An area counts from its own first unit: the first stay comes a Tick after it, however long the timer has been running for another area.</summary>
        public static void CountsFromFirstUnit(FakeWorld world)
        {
            var fake = Fresh();
            var early = Added(world, new LogArea(Square(), 1f));
            var late = Added(world, new LogArea(Square(), 1f));
            var a = UnitAt(world, 10f, 10f);
            var b = UnitAt(world, 20f, 20f);
            fake.Enter(fake.Regions[0], a.Handle);
            world.Advance(0.5f);
            fake.Enter(fake.Regions[1], b.Handle);
            world.Advance(0.5f);
            Expect.Equal(1, early.SinceAtStay.Count, "the first area stays at 1 second");
            Expect.Equal(0, late.SinceAtStay.Count, "the second has not been inside for a second");
            world.Advance(0.5f);
            Expect.Equal(1, late.SinceAtStay.Count, "the second stays a second after its unit entered");
            Expect.Near(1f, late.SinceAtStay[0], "and Since agrees");
        }

        /// <summary>A hidden unit and a loaded one leave at the next place check and enter again when they have a place; a unit that is not inside the region when it is back stays out, and a timer keeps running while only they remain.</summary>
        public static void Hidden(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var hidden = UnitAt(world, 10f, 10f);
            var loaded = UnitAt(world, 20f, 20f);
            var moved = UnitAt(world, 30f, 30f);
            var kept = UnitAt(world, 40f, 40f);
            area.Labels[hidden] = "hidden";
            area.Labels[loaded] = "loaded";
            area.Labels[moved] = "moved";
            area.Labels[kept] = "kept";
            var made = fake.Regions[0];
            fake.Enter(made, hidden.Handle);
            fake.Enter(made, loaded.Handle);
            fake.Enter(made, moved.Handle);
            fake.Enter(made, kept.Handle);
            fake.NoPlace.Add(hidden.Handle);
            fake.NoPlace.Add(loaded.Handle);
            fake.NoPlace.Add(moved.Handle);
            world.Advance(0.21875f);
            Expect.Equal(4, area.Log.Count, "nothing before the pass, 7 steps in");
            world.Advance(0.03125f);
            ExpectLog(area, new[] { "entered hidden", "entered loaded", "entered moved", "entered kept", "left hidden", "left loaded", "left moved" }, "the three leave at the pass, in entering order");
            Expect.True(area.Has(kept), "the one with a place stays");
            fake.Leave(made, kept.Handle);
            Expect.True(AreaTracker.Running, "the three away keep the timer running");
            made.Inside.Remove(moved.Handle);
            fake.NoPlace.Remove(hidden.Handle);
            fake.NoPlace.Remove(moved.Handle);
            world.Advance(0.25f);
            ExpectLog(area, new[] { "entered hidden", "entered loaded", "entered moved", "entered kept", "left hidden", "left loaded", "left moved", "left kept", "entered hidden" }, "the hidden one is back; the moved one is not inside, the loaded one has no place");
            fake.NoPlace.Remove(loaded.Handle);
            world.Advance(0.25f);
            Expect.Equal("entered loaded", area.Log[area.Log.Count - 1], "the loaded one enters at the next pass");
            Expect.Equal(10, area.Log.Count, "and nothing else");
            world.Advance(1f);
            Expect.Equal(10, area.Log.Count, "no more");
        }

        /// <summary>A unit away because it was hidden that dies, or ends, is not entered again, and the timer stops.</summary>
        public static void HiddenThenGone(FakeWorld world)
        {
            var fake = Fresh();
            var area = Added(world, new LogArea(Square()));
            var dies = UnitAt(world, 10f, 10f);
            var ends = UnitAt(world, 20f, 20f);
            area.Labels[dies] = "dies";
            area.Labels[ends] = "ends";
            fake.Enter(fake.Regions[0], dies.Handle);
            fake.Enter(fake.Regions[0], ends.Handle);
            fake.NoPlace.Add(dies.Handle);
            fake.NoPlace.Add(ends.Handle);
            world.Advance(0.25f);
            Expect.Equal(4, area.Log.Count, "both are away");
            fake.Die(dies.Handle);
            ends.Remove();
            fake.NoPlace.Remove(dies.Handle);
            fake.NoPlace.Remove(ends.Handle);
            world.Advance(1f);
            Expect.Equal(4, area.Log.Count, "neither came back");
            Expect.True(!AreaTracker.Running, "and the timer stopped");
        }
    }
}
