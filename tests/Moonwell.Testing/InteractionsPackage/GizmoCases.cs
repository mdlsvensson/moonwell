using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for gizmos and the cycle that ticks them, with the default step of 1/32 second: a tick of 0.1 second is 4 steps.</summary>
    public static class GizmoCases
    {
        /// <summary>Installs a fake interactions world with cells of 256, which the expected cells of these cases count in (the default is 128).</summary>
        private static void Fresh()
        {
            Interactions.Install(new FakeInteractionWorld());
            Interactions.CellSize = 256.0f;
        }

        /// <summary>Moves the game on by a number of steps, one step at a time.</summary>
        private static void Steps(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                world.Advance(Interactions.Step);
            }
        }

        /// <summary>A made gizmo belongs to the owner it was given, or else the current owner; it plays for Neutral Passive unless given a player, it has its place, and its OnCreated ran once.</summary>
        public static void Create(FakeWorld world)
        {
            Fresh();
            var well = Gizmo.Create(new Well(), 10.0f, 20.0f);
            Expect.Same(Runtime.Root, well.Owner, "owned by the current owner, the root");
            Expect.Same(Player.NeutralPassive, well.Player, "Neutral Passive by default");
            Expect.Equal(1, well.Created, "OnCreated ran once");
            Expect.Near(10.0f, well.X, "X");
            Expect.Near(20.0f, well.Y, "Y");
            Expect.Near(0.0f, well.Z, "Z");
            var position = well.Position;
            Expect.Near(10.0f, position.X, "Position.X");
            Expect.Near(20.0f, position.Y, "Position.Y");
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var owned = Gizmo.Create(new Well(), 0.0f, 0.0f, Player.Two, owner);
            Expect.Same(owner, owned.Owner, "owned by the owner given");
            Expect.Same(Player.Two, owned.Player, "plays for the player given");
            owned.Player = Player.Three;
            Expect.Same(Player.Three, owned.Player, "the player can be changed");
            var inside = new Well();
            owner.Run(() => Gizmo.Create(inside, 0.0f, 0.0f));
            Expect.Same(owner, inside.Owner, "owned by the owner that runs the code");
            Expect.Equal(1, owned.Created, "OnCreated ran once for each");
            Expect.Equal(1, inside.Created, "and for the one made inside Run");
        }

        /// <summary>OnCreated and OnLeave run with the gizmo as the current owner.</summary>
        public static void ReactionOwner(FakeWorld world)
        {
            Fresh();
            Owner? created = null;
            Owner? left = null;
            var well = new Well();
            well.Creating = () => { created = Owner.Current; };
            well.Leaving = () => { left = Owner.Current; };
            Gizmo.Create(well, 0.0f, 0.0f);
            Expect.Same(well, created, "Owner.Current inside OnCreated");
            well.Remove();
            Expect.Same(well, left, "Owner.Current inside OnLeave");
            Expect.True(Owner.Current == Runtime.Root, "and the root after");
        }

        /// <summary>A tick of 0.1 second with a step of 1/32 second runs OnTick at steps 4, 8 and 12 as the game moves on.</summary>
        public static void Tick(FakeWorld world)
        {
            Fresh();
            var well = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f);
            var start = Tracking.StepNow;
            Steps(world, 3);
            Expect.Equal(0, well.TickSteps.Count, "no tick before step 4");
            Steps(world, 1);
            Expect.Equal(1, well.TickSteps.Count, "a tick at step 4");
            Steps(world, 4);
            Expect.Equal(2, well.TickSteps.Count, "a tick at step 8");
            Steps(world, 4);
            Expect.Equal(3, well.TickSteps.Count, "a tick at step 12");
            Expect.Equal(start + 4, well.TickSteps[0], "the first tick's step");
            Expect.Equal(start + 8, well.TickSteps[1], "the second tick's step");
            Expect.Equal(start + 12, well.TickSteps[2], "the third tick's step");
        }

        /// <summary>A tick below a step, 0.01 second, runs at every step; a tick that is no whole number of steps is rounded up.</summary>
        public static void TickRounding(FakeWorld world)
        {
            Fresh();
            var fast = Gizmo.Create(new Well(0.01f), 0.0f, 0.0f);
            var slow = Gizmo.Create(new Well(0.11f), 0.0f, 0.0f);
            Steps(world, 8);
            Expect.Equal(8, fast.TickSteps.Count, "0.01 second runs at every step");
            Expect.Equal(2, slow.TickSteps.Count, "0.11 second is 4 steps, so 2 ticks in 8");
            Expect.Equal(1, Cycle.Steps(0.0f), "no time is one step");
            Expect.Equal(1, Cycle.Steps(Interactions.Step), "one step is one step");
            Expect.Equal(2, Cycle.Steps(Interactions.Step * 2.0f), "two steps are two steps");
            Expect.Equal(4, Cycle.Steps(0.1f), "0.1 second is 4 steps");
            Expect.Equal(3, Cycle.Steps(0.0625f + 0.01f), "just over two steps is three");
        }

        /// <summary>A gizmo with no tick, 0, is never ticked; a tick set afterwards starts it, and 0 stops it again.</summary>
        public static void NoTick(FakeWorld world)
        {
            Fresh();
            var plain = Gizmo.Create(new Well(), 0.0f, 0.0f);
            Steps(world, 8);
            Expect.Equal(0, plain.TickSteps.Count, "no tick");
            Expect.Near(0.0f, plain.Tick, "Tick is 0");
            var late = new Well();
            late.Creating = () => late.SetTick(0.1f);
            Gizmo.Create(late, 0.0f, 0.0f);
            Steps(world, 8);
            Expect.Equal(2, late.TickSteps.Count, "a tick set in OnCreated runs from then on");
            late.SetTick(0.0f);
            Steps(world, 8);
            Expect.Equal(2, late.TickSteps.Count, "a tick set to 0 stops it");
            late.SetTick(0.1f);
            Steps(world, 8);
            Expect.Equal(4, late.TickSteps.Count, "and set again starts it again");
        }

        /// <summary>Setting X or Y moves the gizmo's entry to the cell of the new place at once, and Position sets both.</summary>
        public static void Move(FakeWorld world)
        {
            Fresh();
            var well = Gizmo.Create(new Well(), 10.0f, 10.0f);
            Expect.Equal(36, well.entry!.Cell, "the cell of (10, 10)");
            well.X = 300.0f;
            Expect.Equal(37, well.entry!.Cell, "setting X changes the cell at once");
            Expect.Near(300.0f, well.X, "X");
            well.Y = 300.0f;
            Expect.Equal(45, well.entry!.Cell, "setting Y changes the cell at once");
            Expect.Near(300.0f, well.Y, "Y");
            well.Position = new Vector2(-900.0f, -900.0f);
            Expect.Equal(0, well.entry!.Cell, "setting Position changes the cell");
            Expect.Near(-900.0f, well.X, "X from Position");
            Expect.Near(-900.0f, well.Y, "Y from Position");
            well.Z = 5.0f;
            Expect.Near(5.0f, well.Z, "Z");
            var cell = Interactions.Grid.CellList(0);
            Expect.True(cell != null && cell.Count == 1 && cell[0] == well.entry, "the cell lists the gizmo's entry");
            Expect.True(well.entry!.Thing == well, "and the entry stands for the gizmo");
            Expect.Equal(4, well.entry!.Kind, "a gizmo's kind is 4");
        }

        /// <summary>An owner's End ends the gizmo: OnLeave once, out of the grid, and it does not tick again. Remove ends it too, and a gizmo made for an ended owner ends at once.</summary>
        public static void EndsWithOwner(FakeWorld world)
        {
            Fresh();
            var log = new List<string>();
            var owner = new TestOwner("owner", log);
            var well = Gizmo.Create(new Well(0.1f), 10.0f, 10.0f, null, owner);
            var entry = well.entry!;
            Expect.Equal(36, entry.Cell, "in the grid");
            owner.End();
            Expect.True(well.HasEnded, "the gizmo has ended");
            Expect.Equal(1, well.Left, "OnLeave ran once");
            Expect.Equal(-1, entry.Cell, "out of the grid");
            Steps(world, 8);
            Expect.Equal(0, well.TickSteps.Count, "an ended gizmo does not tick");
            well.Remove();
            well.End();
            Expect.Equal(1, well.Left, "ending again does nothing");
            var removed = Gizmo.Create(new Well(), 10.0f, 10.0f);
            removed.Remove();
            Expect.True(removed.HasEnded, "Remove ends it");
            Expect.Equal(1, removed.Left, "OnLeave ran");
            var late = Gizmo.Create(new Well(), 10.0f, 10.0f, null, owner);
            Expect.True(late.HasEnded, "a gizmo made for an owner that has ended ends at once");
            Expect.Equal(-1, late.entry!.Cell, "and is not in the grid");
        }

        /// <summary>A gizmo's OnLeave sees it still in the grid, and it leaves the grid after.</summary>
        public static void LeaveInGrid(FakeWorld world)
        {
            Fresh();
            var cell = -2;
            var well = new Well();
            well.Leaving = () => { cell = well.entry!.Cell; };
            Gizmo.Create(well, 10.0f, 10.0f);
            well.End();
            Expect.Equal(36, cell, "in its cell inside OnLeave");
            Expect.Equal(-1, well.entry!.Cell, "out of it after");
        }

        /// <summary>The cycle's timer runs while a gizmo is tracked, ends when the last gizmo ends, and starts again with the next.</summary>
        public static void CycleStops(FakeWorld world)
        {
            Fresh();
            Expect.True(!Tracking.Running, "no timer before the first gizmo");
            var first = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f);
            var second = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f);
            Expect.True(Tracking.Running, "the timer runs with a gizmo");
            first.End();
            Expect.True(Tracking.Running, "and with the other");
            second.End();
            Expect.True(!Tracking.Running, "it ends with the last");
            world.Advance(1.0f);
            var third = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f);
            Expect.True(Tracking.Running, "it starts again with the next");
            Steps(world, 8);
            Expect.Equal(2, third.TickSteps.Count, "and the next gizmo ticks");
            third.End();
            Expect.True(!Tracking.Running, "and ends again");
        }

        /// <summary>The last gizmo ending in its own OnTick ends the timer, and it does not tick again.</summary>
        public static void EndInTick(FakeWorld world)
        {
            Fresh();
            var well = new Well(0.01f);
            well.Ticking = () => well.End();
            Gizmo.Create(well, 0.0f, 0.0f);
            Steps(world, 2);
            Expect.True(well.HasEnded, "it ended in its tick");
            Expect.Equal(1, well.TickSteps.Count, "it ticked once");
            Expect.True(!Tracking.Running, "the timer ended with it");
            Expect.Equal(1, well.Left, "OnLeave ran once");
        }

        /// <summary>A gizmo made in another's OnTick first ticks at a later step, not the one running.</summary>
        public static void MadeInTick(FakeWorld world)
        {
            Fresh();
            var made = new Well(0.01f);
            var maker = new Well(0.1f);
            maker.Ticking = () =>
            {
                if (made.entry == null)
                {
                    Gizmo.Create(made, 0.0f, 0.0f);
                }
            };
            Gizmo.Create(maker, 0.0f, 0.0f);
            var start = Tracking.StepNow;
            Steps(world, 6);
            Expect.Equal(1, maker.TickSteps.Count, "the maker ticked once");
            Expect.Equal(start + 4, maker.TickSteps[0], "at step 4");
            Expect.Equal(2, made.TickSteps.Count, "the new one ticked at 5 and 6");
            Expect.Equal(start + 5, made.TickSteps[0], "first at the step after the one that made it");
            Expect.Equal(start + 6, made.TickSteps[1], "then at every step");
        }

        /// <summary>A tick of a whole ring of steps or more (256 steps is 8 seconds) is passed over until its step: it ticks at the right step and not before.</summary>
        public static void LongTick(FakeWorld world)
        {
            Fresh();
            var exact = Gizmo.Create(new Well(8.0f), 0.0f, 0.0f);
            var longer = Gizmo.Create(new Well(10.0f), 0.0f, 0.0f);
            var start = Tracking.StepNow;
            Steps(world, 256);
            Expect.Equal(1, exact.TickSteps.Count, "256 steps is one tick");
            Expect.Equal(0, longer.TickSteps.Count, "320 steps has not ticked at 256");
            Expect.Equal(start + 256, exact.TickSteps[0], "exactly at step 256");
            Steps(world, 64);
            Expect.Equal(1, longer.TickSteps.Count, "320 steps is a tick");
            Expect.Equal(start + 320, longer.TickSteps[0], "exactly at step 320");
            longer.End();
            Steps(world, 300);
            Expect.Equal(1, longer.TickSteps.Count, "an ended gizmo does not tick");
            Expect.Equal(2, exact.TickSteps.Count, "and the other goes on");
        }

        /// <summary>
        /// The cycle's lists, which it reads and writes raw in Lua: at a step the gizmos tick first, in order, then the
        /// interactions check; a gizmo ended by an earlier one's tick is passed over and dropped, and the rest keep their
        /// order; each list is in the order things were put on it, so a gizmo made in a tick, which is put on before the one
        /// that made it puts itself on again, ticks before it from the next step, and one that sets its tick in its tick comes
        /// back first at its new step; and a tick of 300 steps waits in its slot as the ring comes round, and ticks exactly at
        /// its step.
        /// </summary>
        public static void CycleLists(FakeWorld world)
        {
            Fresh();
            var log = new List<string>();
            var first = new Well(Interactions.Step);
            var second = new Well(Interactions.Step);
            var third = new Well(Interactions.Step);
            Well? made = null;
            first.Ticking = () => log.Add("1");
            second.Ticking = () => log.Add("2");
            third.Ticking = () => log.Add("3");
            Gizmo.Create(first, 0.0f, 0.0f);
            Gizmo.Create(second, 0.0f, 0.0f);
            Gizmo.Create(third, 0.0f, 0.0f);
            var holder = Gizmo.Create(new Well(), 0.0f, 0.0f);
            var aura = new Healing();
            holder.Interact(aura);
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"));
            world.MoveUnit(handle, 100.0f, 0.0f);
            Unit.Of(handle);
            aura.Meeting = pair => log.Add("check");
            var longer = Gizmo.Create(new Well(300.0f * Interactions.Step), 0.0f, 0.0f);
            var start = Tracking.StepNow;
            Steps(world, 1);
            Expect.Equal(string.Join(",", new[] { "1", "2", "3", "check" }), string.Join(",", log), "the gizmos tick in order, then the interaction checks");
            log.Clear();
            second.Ticking = () =>
            {
                log.Add("2");
                third.End();
                made = Gizmo.Create(new Well(Interactions.Step), 0.0f, 0.0f);
                made.Ticking = () => log.Add("m");
                second.Ticking = () => log.Add("2");
            };
            first.Ticking = () =>
            {
                log.Add("1");
                first.SetTick(Interactions.Step * 2.0f);
                first.Ticking = () => log.Add("1");
            };
            Steps(world, 1);
            Expect.Equal("1,2", string.Join(",", log), "the ended one is passed over, the one made does not tick in the step");
            log.Clear();
            Steps(world, 1);
            Expect.Equal("m,2", string.Join(",", log), "the one whose tick grew waits; the one made was put on before the one that made it");
            log.Clear();
            Steps(world, 1);
            Expect.Equal("1,m,2", string.Join(",", log), "and it comes back first, as it was put on for this step in its tick, before the others ticked");
            Steps(world, 300 - 4);
            Expect.Equal(1, longer.TickSteps.Count, "the tick of 300 steps came round the ring once");
            Expect.Equal(start + 300, longer.TickSteps[0], "and ticked exactly at its step");
            Expect.Equal(1, third.TickSteps.Count, "the ended one ticked only before it ended");
            Expect.True(made != null && made.TickSteps.Count == 298, "the one made ticked at every step since");
        }

        /// <summary>A throwing OnTick is reported as Well.OnTick, and the next gizmo still ticks, as does the one that threw at the next tick.</summary>
        public static void ThrowingTick(FakeWorld world)
        {
            Fresh();
            var first = new Well(0.1f);
            first.Throws = true;
            Gizmo.Create(first, 0.0f, 0.0f);
            var second = Gizmo.Create(new Well(0.1f), 0.0f, 0.0f);
            Steps(world, 8);
            Expect.Equal(2, first.TickSteps.Count, "the one that threw ticks again");
            Expect.Equal(2, second.TickSteps.Count, "the next one ticked each time");
            Expect.True(Logged(world, "Well.OnTick failed"), "a logged line names Well.OnTick");
        }

        /// <summary>OnTick runs with the gizmo as the current owner, so what it makes is the gizmo's.</summary>
        public static void TickOwner(FakeWorld world)
        {
            Fresh();
            Owner? seen = null;
            var well = new Well(0.01f);
            well.Ticking = () => { seen = Owner.Current; };
            Gizmo.Create(well, 0.0f, 0.0f);
            Steps(world, 1);
            Expect.Same(well, seen, "Owner.Current inside OnTick");
            Expect.True(Owner.Current == Runtime.Root, "and the root after");
        }

        /// <summary>A throwing OnCreated or OnLeave is reported, and the gizmo still comes in and leaves.</summary>
        public static void ThrowingReactions(FakeWorld world)
        {
            Fresh();
            var well = new Well();
            well.Creating = () => { throw new InvalidOperationException("boom"); };
            well.Leaving = () => { throw new InvalidOperationException("boom"); };
            Gizmo.Create(well, 10.0f, 10.0f);
            Expect.Equal(36, well.entry!.Cell, "a gizmo whose OnCreated throws is in the grid");
            well.End();
            Expect.True(well.HasEnded, "and ends");
            Expect.Equal(-1, well.entry!.Cell, "and leaves the grid, though its OnLeave threw");
            Expect.True(Logged(world, "Well.OnCreated failed"), "OnCreated is reported");
            Expect.True(Logged(world, "Well.OnLeave failed"), "OnLeave is reported");
        }

        /// <summary>Whether a line of the log has the text.</summary>
        private static bool Logged(FakeWorld world, string text)
        {
            foreach (var line in world.Logged)
            {
                if (line.Contains(text))
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>A map's gizmo may have fields of its own named as the package's (z, player, tick, due): each stays its own, in Lua too, and the gizmo works as any other.</summary>
        public static void ShadowedFields(FakeWorld world)
        {
            Fresh();
            var well = Gizmo.Create(new ShadowWell(), 0.0f, 0.0f);
            Expect.Same(Player.NeutralPassive, well.Player, "it plays for Neutral Passive");
            Expect.True(well.OwnPlayer == null, "its own player is none");
            Expect.Near(0.0f, well.Z, "Z is 0");
            well.Z = 2.0f;
            Expect.Near(2.0f, well.Z, "Z is set");
            Expect.Near(5.0f, well.OwnZ, "its own height is 5");
            Expect.Near(Interactions.Step, well.Tick, "Tick is a step");
            Expect.Near(3.0f, well.OwnTick, "its own tick is 3");
            Expect.Equal(9, well.OwnDue, "its own due is 9");
            Steps(world, 2);
            Expect.Equal(2, well.Ticks, "it ticks at every step");
        }
    }
}
