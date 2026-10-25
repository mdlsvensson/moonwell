using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for interactions, on the default settings: a step of 1/32 second, so a FireAura's tick of half a second is 16 steps and the first check comes 16 steps after it starts.</summary>
    public static class InteractionCases
    {
        /// <summary>Installs a fake interactions world and clears the test log.</summary>
        private static FakeInteractionWorld Fresh()
        {
            var fake = new FakeInteractionWorld();
            Interactions.Install(fake);
            TestLog.Lines.Clear();
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

        /// <summary>Makes a LeaveLogger, a unit that logs "leave" in its OnLeave, at a place and takes it in.</summary>
        private static Unit LoggerAt(FakeWorld world, float x, float y)
        {
            ObjectTypes.Unit(ObjectTypes.FourCC("h004"), () => new LeaveLogger());
            var handle = world.NewUnit(ObjectTypes.FourCC("h004"));
            world.MoveUnit(handle, x, y);
            return Unit.Of(handle)!;
        }

        /// <summary>Checks that the test log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>How many lines of the test log equal a line.</summary>
        private static int CountOf(string line)
        {
            var count = 0;
            for (var i = 0; i < TestLog.Lines.Count; i++)
            {
                if (TestLog.Lines[i] == line)
                {
                    count++;
                }
            }

            return count;
        }

        /// <summary>Whether a line of the game's log has the text.</summary>
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

        /// <summary>A unit walks into the aura's range: OnMeet then OnTick in one check; it ticks every half second; it walks out: OnPart once.</summary>
        public static void MeetTickPart(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 500.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.Same(aura, a.Interact(aura), "Interact gives the interaction back");
            Expect.Same(a, aura.Holder, "its holder");
            Expect.Same(Runtime.Root, aura.Owner, "it belongs to the current owner, the root");
            Steps(world, 16);
            Expect.Equal(0, TestLog.Lines.Count, "b is out of range at the first check");
            world.MoveUnit(b.Handle, 200.0f, 0.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "b walked in: OnMeet, then OnTick in the same check");
            Expect.Equal(1, aura.Pairs.Count, "one pair");
            var pair = aura.Pairs[0];
            Expect.Same(a, pair.Holder, "the pair's holder");
            Expect.Same(b, pair.Other, "the pair's other");
            Steps(world, 15);
            Expect.Equal(2, TestLog.Lines.Count, "no tick before half a second");
            Steps(world, 1);
            ExpectLog(new string[] { "meet b", "tick b", "tick b" }, "a tick after half a second");
            world.MoveUnit(b.Handle, 500.0f, 0.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b" }, "b walked out: OnPart");
            Steps(world, 32);
            Expect.Equal(4, TestLog.Lines.Count, "nothing more");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Expect.True(pair.HasEnded, "the pair has ended");
            Expect.True(!aura.HasEnded, "the aura goes on");
        }

        /// <summary>A unit exactly at the range meets; one just past it does not.</summary>
        public static void ExactRange(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 300.0f, 0.0f);
            var c = UnitAt(world, 0.0f, 300.5f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "at 300 it meets, at 300.5 not");
        }

        /// <summary>A unit Accepts refuses makes no pair and is asked again at the next tick; Accepts runs with the interaction as the current owner, and a pair is not asked again.</summary>
        public static void Accepts(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            aura.Accepting = unit => false;
            a.Interact(aura);
            Steps(world, 16);
            Expect.Equal(1, aura.Asked, "asked once");
            Expect.Same(aura, aura.AskedAs, "with the interaction as the current owner");
            Expect.Equal(0, aura.Pairs.Count, "refused: no pair");
            Expect.Equal(0, TestLog.Lines.Count, "and no reaction");
            Steps(world, 16);
            Expect.Equal(2, aura.Asked, "asked again at the next tick");
            aura.Accepting = null;
            Steps(world, 16);
            Expect.Equal(3, aura.Asked, "asked a third time");
            ExpectLog(new string[] { "meet b", "tick b" }, "accepted: they meet");
            Steps(world, 16);
            Expect.Equal(3, aura.Asked, "a pair is not asked again");
        }

        /// <summary>An interaction never pairs with its own holder.</summary>
        public static void NotHolder(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var aura = new FireAura();
            a.Interact(aura);
            Steps(world, 32);
            Expect.Equal(0, aura.Asked, "the holder is not asked about");
            Expect.Equal(0, aura.Pairs.Count, "and makes no pair");
            var b = UnitAt(world, 100.0f, 0.0f);
            Steps(world, 16);
            Expect.Equal(1, aura.Pairs.Count, "a unit near it does");
            Expect.Same(b, aura.Pairs[0].Other, "with that unit");
            Expect.Equal(1, aura.Asked, "and only that unit was asked about");
        }

        /// <summary>Two units with an aura each burn each other through two pairs, one of each aura.</summary>
        public static void TwoWays(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var first = new FireAura();
            first.Tag = unit => unit == b ? "b" : "?";
            var second = new FireAura();
            second.Tag = unit => unit == a ? "a" : "?";
            a.Interact(first);
            b.Interact(second);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet a", "tick a" }, "each aura meets the other unit, in the order they were made");
            Expect.Same(b, first.Pairs[0].Other, "a's aura pairs with b");
            Expect.Same(a, second.Pairs[0].Other, "b's aura pairs with a");
        }

        /// <summary>An aura made in a buff's OnAdded is the buff's: the buff's expiry ends it and parts its pairs.</summary>
        public static void OwnedByBuff(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var buff = a.Apply(new Inferno());
            var aura = buff.Aura;
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.Same(buff, aura.Owner, "the buff's");
            Expect.Same(a, aura.Holder, "held by the buff's unit");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "it meets b");
            Steps(world, 24);
            Expect.True(buff.HasEnded, "the buff expired");
            Expect.True(aura.HasEnded, "the aura ended with it");
            Expect.Equal(1, CountOf("part b"), "the pair parted once");
            Expect.Equal("part b", TestLog.Lines[TestLog.Lines.Count - 1], "last");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            var lines = TestLog.Lines.Count;
            Steps(world, 32);
            Expect.Equal(lines, TestLog.Lines.Count, "an ended aura does not check");
        }

        /// <summary>The holder removed: its interaction ends and the pairs part, newest first, before the holder's OnLeave.</summary>
        public static void HolderEnds(FakeWorld world)
        {
            Fresh();
            var a = LoggerAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "it meets b, then c");
            TestLog.Lines.Clear();
            a.Remove();
            ExpectLog(new string[] { "part c", "part b", "leave" }, "the pairs part, newest first, before OnLeave");
            Expect.True(aura.HasEnded, "the aura ended");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Steps(world, 32);
            Expect.Equal(3, TestLog.Lines.Count, "nothing more");
        }

        /// <summary>The other removed: its pair parts at once, before the other's OnLeave, and the other pairs go on.</summary>
        public static void OtherEnds(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = LoggerAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            a.Interact(aura);
            Steps(world, 16);
            Expect.Equal(2, aura.Pairs.Count, "two pairs");
            TestLog.Lines.Clear();
            b.Remove();
            ExpectLog(new string[] { "part b", "leave" }, "OnPart at once, before b's OnLeave");
            Expect.Equal(1, aura.Pairs.Count, "one pair left");
            Expect.Same(c, aura.Pairs[0].Other, "with c");
            Steps(world, 16);
            ExpectLog(new string[] { "part b", "leave", "tick c" }, "c still ticks");
        }

        /// <summary>The holder dies: at the next check its pairs part, and none meets while it is dead; revived, they meet again.</summary>
        public static void HolderNoPlace(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "they meet");
            a.Kill();
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "the dead holder's pair parts at the check");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Steps(world, 32);
            Expect.Equal(3, TestLog.Lines.Count, "none meets while it is dead");
            Expect.True(!aura.HasEnded, "the aura goes on");
            world.ReviveAt(a.Handle, 0.0f, 0.0f, false);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b", "meet b", "tick b" }, "revived: they meet again");
        }

        /// <summary>A gizmo holder ended inside OnMeet: no OnTick after it, the check stops, and each pair parts once.</summary>
        public static void EndsInMeet(FakeWorld world)
        {
            Fresh();
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var d = UnitAt(world, 250.0f, 0.0f);
            var well = Gizmo.Create(new Well(), 0.0f, 0.0f);
            var healing = new Healing();
            healing.Tag = unit => unit == b ? "b" : unit == c ? "c" : unit == d ? "d" : "?";
            healing.Meeting = pair =>
            {
                if (pair.Other == c)
                {
                    well.End();
                }
            };
            Expect.Same(healing, well.Interact(healing), "a gizmo's Interact gives the interaction back");
            Expect.Same(well, healing.Holder, "held by the well");
            Steps(world, 1);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "part c", "part b" }, "no OnTick after the holder ended, nor a meeting with d; newest first");
            Expect.True(well.HasEnded, "the well ended");
            Expect.True(healing.HasEnded, "and its healing");
            Expect.Equal(1, well.Left, "the well's OnLeave ran once");
            Steps(world, 8);
            Expect.Equal(5, TestLog.Lines.Count, "nothing more");
        }

        /// <summary>An interaction that ends itself in an OnTick: no other pair ticks after, and its pairs part once, newest first.</summary>
        public static void EndsInTick(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            a.Interact(aura);
            Steps(world, 16);
            aura.Ticking = pair => aura.End();
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "tick b", "part c", "part b" }, "c does not tick after the aura ended");
            Expect.True(aura.HasEnded, "the aura ended");
            Steps(world, 16);
            Expect.Equal(7, TestLog.Lines.Count, "nothing more");
        }

        /// <summary>An other removed inside another pair's OnTick parts at once and does not tick in that check.</summary>
        public static void OtherEndsInTick(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            a.Interact(aura);
            Steps(world, 16);
            aura.Ticking = pair =>
            {
                if (pair.Other == b)
                {
                    c.Remove();
                }
            };
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "tick b", "part c" }, "c parts and does not tick");
            Expect.Equal(1, aura.Pairs.Count, "b's pair is left");
            Steps(world, 16);
            Expect.Equal(7, TestLog.Lines.Count, "b goes on ticking");
        }

        /// <summary>A tick of 0.1 second with a step of 1/32 second checks at steps 4, 8 and 12.</summary>
        public static void Interval(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var aura = a.Interact(new FireAura(300.0f, 0.1f));
            var start = Tracking.StepNow;
            Steps(world, 12);
            Expect.Equal(3, aura.TickSteps.Count, "three checks in 12 steps");
            Expect.Equal(start + 4, aura.TickSteps[0], "the first at step 4");
            Expect.Equal(start + 8, aura.TickSteps[1], "then 8");
            Expect.Equal(start + 12, aura.TickSteps[2], "then 12");
        }

        /// <summary>A paused interaction makes no checks and keeps its pairs; resumed, it checks again.</summary>
        public static void PausedInteraction(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "they meet");
            aura.Pause();
            Expect.True(aura.IsPaused, "paused");
            Steps(world, 32);
            Expect.Equal(2, TestLog.Lines.Count, "no ticks while paused");
            world.MoveUnit(b.Handle, 600.0f, 0.0f);
            Steps(world, 32);
            Expect.Equal(2, TestLog.Lines.Count, "no checks: b out of range does not part");
            Expect.Equal(1, aura.Pairs.Count, "the pair is kept");
            aura.Resume();
            Expect.True(!aura.IsPaused, "resumed");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "the next check parts b");
        }

        /// <summary>A throwing OnTick is reported as FireAura.OnTick, and the other pairs still tick, as does the one that threw at the next tick.</summary>
        public static void ErrorReported(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            aura.Throws = b;
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "c meets after b's OnTick threw");
            Expect.True(Logged(world, "FireAura.OnTick failed"), "a logged line names FireAura.OnTick");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "tick b", "tick c" }, "both tick again");
        }

        /// <summary>An interaction started inside OnMeet first checks at a later step, not the one running, and belongs to the pair, the current owner; a Find in the same OnMeet does not change what the check walks.</summary>
        public static void MadeInReaction(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var first = new FireAura();
            first.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            var second = new FireAura(300.0f, Interactions.Step);
            var madeAt = -1;
            var found = -1;
            Owner? maker = null;
            first.Meeting = pair =>
            {
                if (madeAt < 0)
                {
                    madeAt = Tracking.StepNow;
                    maker = pair;
                    b.Interact(second);
                    found = Find.InRange<Unit>(0.0f, 0.0f, 500.0f).Count;
                }
            };
            a.Interact(first);
            Steps(world, 16);
            Expect.True(madeAt >= 0, "made in the first aura's OnMeet");
            Expect.Equal(3, found, "the Find inside OnMeet found the three units");
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "the check went on to c after the Find");
            Expect.Equal(0, Find.Pending, "and nothing is left on the shared lists");
            Expect.Equal(0, second.TickSteps.Count, "the step that made it did not check it");
            Expect.Same(maker, second.Owner, "it belongs to the pair");
            Steps(world, 1);
            Expect.Equal(2, second.TickSteps.Count, "it checks at the next step, and meets a and c");
            Expect.Equal(madeAt + 1, second.TickSteps[0], "the step after the one that made it");
            Expect.Same(a, second.Pairs[0].Other, "and pairs with a");
        }

        /// <summary>Pairs is one view of the pairs, in meeting order, by index and by foreach, that follows the pairs as they part.</summary>
        public static void PairsView(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = a.Interact(new FireAura());
            Expect.Equal(0, aura.Pairs.Count, "none before the first check");
            Steps(world, 16);
            var pairs = aura.Pairs;
            Expect.Same(pairs, aura.Pairs, "the same view each time");
            Expect.Equal(2, pairs.Count, "two pairs");
            Expect.Same(b, pairs[0].Other, "b met first");
            Expect.Same(c, pairs[1].Other, "then c");
            var others = new List<Unit>();
            foreach (var pair in aura.Pairs)
            {
                others.Add(pair.Other);
            }

            Expect.Equal(2, others.Count, "foreach gives both");
            Expect.Same(b, others[0], "b first");
            Expect.Same(c, others[1], "then c");
            b.Remove();
            Expect.Equal(1, pairs.Count, "the view follows a parting");
            Expect.Same(c, pairs[0].Other, "c is left");
        }

        /// <summary>Interact checks in a development build that the holder is of the interaction's holder class, that Range is more than 0, and that the interaction was not started before.</summary>
        public static void InteractChecks(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
#if DEBUG
            Expect.Throws<MoonwellException>(() => a.Interact(new FireAura(0.0f, 0.5f)), "a Range of 0");
            Expect.Throws<MoonwellException>(() => a.Interact(new Healing()), "a unit holding an interaction for wells");
            Expect.Throws<MoonwellException>(() => new Well().Interact(new Healing()), "a gizmo that was never created");
            var aura = a.Interact(new FireAura());
            Expect.Throws<MoonwellException>(() => b.Interact(aura), "an interaction started twice");
            Expect.Same(a, aura.Holder, "the first start stands");
#endif
            Expect.True(b.IsAlive, "b is untouched");
        }

        /// <summary>A unit that starts an aura in its OnCreated while units are tracked holds it: the runtime tells the package of the unit only after OnCreated, so Interact tracks it then, and the hook after changes nothing.</summary>
        public static void InteractInCreated(FakeWorld world)
        {
            Fresh();
            var b = UnitAt(world, 100.0f, 0.0f);
            Tracking.Need(Tracking.UnitKind);
            ObjectTypes.Unit(ObjectTypes.FourCC("h005"), () => new AuraBearer());
            var handle = world.NewUnit(ObjectTypes.FourCC("h005"));
            world.MoveUnit(handle, 0.0f, 0.0f);
            var a = (AuraBearer)Unit.Of(handle)!;
            var aura = a.Aura;
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.True(!aura.HasEnded, "the aura started in OnCreated goes on");
            Expect.Same(a, aura.Holder, "held by the unit");
            Expect.Same(a, aura.Owner, "and the unit's, the current owner in OnCreated");
            Expect.Equal(2, Tracking.Count, "the unit is tracked once");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "it meets b");
        }

        /// <summary>A map's interaction may have fields of its own named as the package's (paused, range, tick, started, due, holder): each stays its own, in Lua too, and the interaction works as any other.</summary>
        public static void ShadowedFields(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            UnitAt(world, 100.0f, 0.0f);
            var aura = new ShadowAura();
            Expect.True(!aura.IsPaused, "the interaction is not paused");
            Expect.True(aura.OwnPaused, "its own paused is true");
            Expect.Near(300.0f, aura.Range, "Range is 300");
            Expect.Near(1.0f, aura.OwnRange, "its own range is 1");
            Expect.Near(0.5f, aura.Tick, "Tick is half a second");
            Expect.Near(2.0f, aura.OwnTick, "its own tick is 2");
            aura.Resume();
            a.Interact(aura);
            Expect.Same(a, aura.Holder, "Holder is the unit");
            Expect.True(aura.OwnHolder == null, "its own holder is none");
            Expect.True(aura.OwnStarted, "its own started is true");
            Expect.Equal(7, aura.OwnDue, "its own due is 7");
            Steps(world, 16);
            ExpectLog(new string[] { "meet" }, "it meets at its first check");
        }
    }
}
