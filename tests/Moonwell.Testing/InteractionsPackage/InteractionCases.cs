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
            Expect.Same(aura, a.Interact(aura, Game.Instance), "Interact gives the interaction back");
            Expect.Same(a, aura.Holder, "its holder");
            Expect.Same(Game.Instance, aura.Parent, "it belongs to the scope given, the game");
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
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "at 300 it meets, at 300.5 not");
        }

        /// <summary>A unit Accepts refuses makes no pair and is asked again at the next tick, and a pair is asked again before each tick (decision 73).</summary>
        public static void Accepts(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            aura.Accepting = unit => false;
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            Expect.Equal(1, aura.Asked, "asked once");
            Expect.Equal(0, aura.Pairs.Count, "refused: no pair");
            Expect.Equal(0, TestLog.Lines.Count, "and no reaction");
            Steps(world, 16);
            Expect.Equal(2, aura.Asked, "asked again at the next tick");
            aura.Accepting = null;
            Steps(world, 16);
            Expect.Equal(3, aura.Asked, "asked a third time");
            ExpectLog(new string[] { "meet b", "tick b" }, "accepted: they meet");
            Steps(world, 16);
            Expect.Equal(4, aura.Asked, "a pair is asked again before each tick");
        }

        /// <summary>An interaction never pairs with its own holder.</summary>
        public static void NotHolder(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var aura = new FireAura();
            a.Interact(aura, Game.Instance);
            Steps(world, 32);
            Expect.Equal(0, aura.Asked, "the holder is not asked about");
            Expect.Equal(0, aura.Pairs.Count, "and makes no pair");
            var b = UnitAt(world, 100.0f, 0.0f);
            Steps(world, 16);
            Expect.Equal(1, aura.Pairs.Count, "a unit near it does");
            Expect.Same(b, aura.Pairs[0].Other, "with that unit");
            Expect.Equal(1, aura.Asked, "and only that unit was asked about");
        }

        /// <summary>Two units with an aura each burn each other through two pairs, one of each aura; the second aura, started after the first with the same tick, first checks a step sooner.</summary>
        public static void TwoWays(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var first = new FireAura();
            first.Tag = unit => unit == b ? "b" : "?";
            var second = new FireAura();
            second.Tag = unit => unit == a ? "a" : "?";
            a.Interact(first, Game.Instance);
            b.Interact(second, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet a", "tick a", "meet b", "tick b" }, "each aura meets the other unit, the second a step before the first");
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
            Expect.Same(buff, aura.Parent, "the buff's");
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
            a.Interact(aura, Game.Instance);
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
            a.Interact(aura, Game.Instance);
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
            a.Interact(aura, Game.Instance);
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
            var well = Gizmo.Create(new Well(), 0.0f, 0.0f, Game.Instance);
            var healing = new Healing();
            healing.Tag = unit => unit == b ? "b" : unit == c ? "c" : unit == d ? "d" : "?";
            healing.Meeting = pair =>
            {
                if (pair.Other == c)
                {
                    well.End();
                }
            };
            Expect.Same(healing, well.Interact(healing, Game.Instance), "a gizmo's Interact gives the interaction back");
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
            a.Interact(aura, Game.Instance);
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
            a.Interact(aura, Game.Instance);
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
            var aura = a.Interact(new FireAura(300.0f, 0.1f), Game.Instance);
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
            a.Interact(aura, Game.Instance);
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
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "c meets after b's OnTick threw");
            Expect.True(Logged(world, "FireAura.OnTick failed"), "a logged line names FireAura.OnTick");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "tick b", "tick c" }, "both tick again");
        }

        /// <summary>An interaction started inside OnMeet first checks at a later step, not the one running, and belongs to the pair, the scope its line names; a Find in the same OnMeet does not change what the check walks.</summary>
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
            Scope? maker = null;
            first.Meeting = pair =>
            {
                if (madeAt < 0)
                {
                    madeAt = Tracking.StepNow;
                    maker = pair;
                    b.Interact(second, pair);
                    found = Find.InRange<Unit>(0.0f, 0.0f, 500.0f).Count;
                }
            };
            a.Interact(first, Game.Instance);
            Steps(world, 16);
            Expect.True(madeAt >= 0, "made in the first aura's OnMeet");
            Expect.Equal(3, found, "the Find inside OnMeet found the three units");
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "the check went on to c after the Find");
            Expect.Equal(0, Find.Pending, "and nothing is left on the shared lists");
            Expect.Equal(0, second.TickSteps.Count, "the step that made it did not check it");
            Expect.Same(maker, second.Parent, "it belongs to the pair");
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
            Expect.True(new FireAura().view != null, "the view is made with the interaction, so no read makes it");
            var aura = a.Interact(new FireAura(), Game.Instance);
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
            Expect.Throws<MoonwellException>(() => a.Interact(new FireAura(0.0f, 0.5f), Game.Instance), "a Range of 0");
            Expect.Throws<MoonwellException>(() => a.Interact(new Healing(), Game.Instance), "a unit holding an interaction for wells");
            Expect.Throws<MoonwellException>(() => new Well().Interact(new Healing(), Game.Instance), "a gizmo that was never created");
            var aura = a.Interact(new FireAura(), Game.Instance);
            Expect.Throws<MoonwellException>(() => b.Interact(aura, Game.Instance), "an interaction started twice");
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
            Expect.Same(a, aura.Parent, "and the unit's, the scope its line in OnCreated names");
            Expect.Equal(2, Tracking.Count, "the unit is tracked once");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "it meets b");
        }

        /// <summary>An item on the ground holds an interaction through Interact, and the interaction ends with the item.</summary>
        public static void ItemHolder(FakeWorld world)
        {
            Fresh();
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, 0.0f, 0.0f);
            world.PlacedItems.Add(handle);
            var item = Item.Of(handle)!;
            var b = UnitAt(world, 100.0f, 0.0f);
            var shine = new Shine();
            shine.Tag = unit => unit == b ? "b" : "?";
            Expect.Same(shine, item.Interact(shine, Game.Instance), "Interact gives the interaction back");
            Expect.Same(item, shine.Holder, "held by the item");
            Steps(world, 1);
            ExpectLog(new string[] { "meet b" }, "the item meets b");
            item.Remove();
            ExpectLog(new string[] { "meet b", "part b" }, "and parts with it when it is removed");
            Expect.True(shine.HasEnded, "the interaction ended with the item");
        }

        /// <summary>A destructable holds an interaction through Interact, and the interaction ends with the destructable.</summary>
        public static void DestructableHolder(FakeWorld world)
        {
            Fresh();
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(handle, 0.0f, 0.0f);
            world.PlacedDestructables.Add(handle);
            var barrel = Destructable.Of(handle)!;
            var b = UnitAt(world, 100.0f, 0.0f);
            var shine = new Shine();
            shine.Tag = unit => unit == b ? "b" : "?";
            Expect.Same(shine, barrel.Interact(shine, Game.Instance), "Interact gives the interaction back");
            Expect.Same(barrel, shine.Holder, "held by the destructable");
            Steps(world, 1);
            ExpectLog(new string[] { "meet b" }, "the destructable meets b");
            barrel.Remove();
            ExpectLog(new string[] { "meet b", "part b" }, "and parts with it when it is removed");
            Expect.True(shine.HasEnded, "the interaction ended with the destructable");
        }

        /// <summary>Pause inside a reaction stops the check it is in: the pair that met does not tick and no other meets, until Resume.</summary>
        public static void PauseInReaction(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            aura.Meeting = pair => aura.Pause();
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b" }, "b met, and the check stopped there");
            aura.Meeting = null;
            aura.Resume();
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c" }, "resumed, b ticks and c meets");
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
            a.Interact(aura, Game.Instance);
            Expect.Same(a, aura.Holder, "Holder is the unit");
            Expect.True(aura.OwnHolder == null, "its own holder is none");
            Expect.True(aura.OwnStarted, "its own started is true");
            Expect.Equal(7, aura.OwnDue, "its own due is 7");
            Steps(world, 16);
            ExpectLog(new string[] { "meet" }, "it meets at its first check");
        }

        /// <summary>
        /// The check's inner loops, which in Lua read and write the package's lists raw: things in five cells around the holder
        /// meet in the grid's order, the cells row by row from the bottom and a cell's things in the order they came in, with
        /// the ones out of range, the holder and one far away left out; the pairs tick and part in meeting order; a Find inside
        /// a reaction leaves the check's things as they were; a stopped other is forgotten when it leaves range while another
        /// stays stopped; and the shared candidates and walk are empty after every check.
        /// </summary>
        public static void InnerLoops(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var near = UnitAt(world, 0.0f, -350.0f);
            var south = UnitAt(world, 0.0f, -200.0f);
            var west = UnitAt(world, -200.0f, 0.0f);
            var beside = UnitAt(world, 100.0f, 100.0f);
            var past = UnitAt(world, 310.0f, 0.0f);
            var east = UnitAt(world, 200.0f, 0.0f);
            var north = UnitAt(world, 0.0f, 200.0f);
            UnitAt(world, 1000.0f, 1000.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == south ? "s" : unit == west ? "w" : unit == beside ? "b" : unit == east ? "e" : unit == north ? "n" : unit == near ? "near" : unit == past ? "past" : "?";
            var found = -1;
            aura.Ticking = pair =>
            {
                if (pair.Other == south && found < 0)
                {
                    found = Find.InRange<Unit>(0.0f, 0.0f, 2000.0f).Count;
                }
            };
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet s", "tick s", "meet w", "tick w", "meet b", "tick b", "meet e", "tick e", "meet n", "tick n" }, "they meet in the grid's order");
            Expect.Equal(9, found, "the Find inside a reaction found all nine units");
            Expect.Equal(0, Find.Pending, "no candidate is left");
            Expect.Equal(0, Pairing.Walk.Count, "no pair is left on the walk");
            TestLog.Lines.Clear();
            world.MoveUnit(west.Handle, -500.0f, 0.0f);
            world.MoveUnit(beside.Handle, 500.0f, 500.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "part w", "part b", "tick s", "tick e", "tick n" }, "the ones gone part, then the rest tick, in meeting order");
            TestLog.Lines.Clear();
            aura.Ticking = pair =>
            {
                if (pair.Other == south || pair.Other == north)
                {
                    pair.Stop();
                }
            };
            Steps(world, 16);
            ExpectLog(new string[] { "tick s", "part s", "tick e", "tick n", "part n" }, "two pairs stop in their ticks");
            TestLog.Lines.Clear();
            aura.Ticking = null;
            world.MoveUnit(south.Handle, 0.0f, -900.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "tick e" }, "the stopped one still in range does not meet");
            TestLog.Lines.Clear();
            world.MoveUnit(south.Handle, 0.0f, -200.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "tick e", "meet s", "tick s" }, "the one that left range meets again; the other is still stopped");
            Expect.Equal(0, Find.Pending, "no candidate is left at the end");
            Expect.Equal(0, Pairing.Walk.Count, "and no pair on the walk");
        }

        /// <summary>
        /// Each reaction runs guarded, in Lua through one xpcall each with no closure: an Accepts that throws is reported as
        /// FireAura.Accepts and refuses, and the other is asked again at the next tick; an OnMeet that fails on a null (a Lua
        /// error in Lua) is reported as FireAura.OnMeet and the pair still ticks; an
        /// OnPart that throws is reported as FireAura.OnPart and the pair still parts; and the check goes on to the next other
        /// each time.
        /// </summary>
        public static void GuardedReactions(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : "?";
            var refuseC = true;
            aura.Accepting = unit =>
            {
                if (unit == c && refuseC)
                {
                    throw new System.InvalidOperationException("no");
                }

                return true;
            };
            Unit? none = null;
            aura.Meeting = pair =>
            {
                if (pair.Other == b)
                {
                    TestLog.Lines.Add("x " + none!.X);
                }
            };
            Scope? metB = null;
            aura.Ticking = pair =>
            {
                if (pair.Other == b && metB == null)
                {
                    metB = pair;
                }
            };
            aura.Parting = pair => { throw new System.InvalidOperationException("part"); };
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b" }, "b meets though its OnMeet failed, and c's Accepts threw");
            Expect.True(Logged(world, "FireAura.Accepts failed"), "the Accepts that threw is reported");
            Expect.True(Logged(world, "FireAura.OnMeet failed"), "the OnMeet that failed on a null is reported");
            Expect.True(metB != null, "b ticked");
            Expect.Equal(2, aura.Asked, "both were asked");
            refuseC = false;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "meet c", "tick c" }, "c is asked again and meets");
            world.MoveUnit(b.Handle, 900.0f, 0.0f);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "meet c", "tick c", "part b", "tick c" }, "b parts though its OnPart threw, and c ticks after");
            Expect.True(Logged(world, "FireAura.OnPart failed"), "the OnPart that threw is reported");
            Expect.Equal(1, aura.Pairs.Count, "b's pair is gone");
        }

        /// <summary>A check that fails outside a reaction, here the world's read of a destructable, takes what it added off the shared candidates and walk, and its error is reported as Scorch.Check; nothing parts, an interaction due after it in the same step still checks in that step, and the one that failed checks again at its next tick.</summary>
        public static void CheckFails(FakeWorld world)
        {
            var fake = Fresh();
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(handle, 100.0f, 0.0f);
            world.PlacedDestructables.Add(handle);
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 900.0f, 0.0f);
            var scorch = new Scorch();
            a.Interact(scorch, Game.Instance);
            var shine = new Shine();
            shine.Tag = unit => unit == b ? "b" : "?";
            a.Interact(shine, Game.Instance);
            Steps(world, 1);
            ExpectLog(new string[] { "meet", "tick" }, "the scorch meets the destructable");
            fake.Failing.Add(handle);
            world.MoveUnit(b.Handle, 100.0f, 0.0f);
            Steps(world, 1);
            ExpectLog(new string[] { "meet", "tick", "meet b" }, "the failed check ran no reaction, and the shine after it checked in the same step");
            Expect.True(Logged(world, "Scorch.Check failed"), "the error is reported as Scorch.Check");
            Expect.True(!Logged(world, ".Timer failed"), "and nothing went on up to the cycle's timer");
            Expect.Equal(0, Find.Pending, "no candidate is left");
            Expect.Equal(0, Pairing.Walk.Count, "no pair is left on the walk");
            Expect.Equal(1, scorch.Pairs.Count, "the pair is kept");
            fake.Failing.Clear();
            Steps(world, 1);
            ExpectLog(new string[] { "meet", "tick", "meet b", "tick" }, "the scorch checks again at its next tick");
        }

        /// <summary>A check takes in the things of its class of other: Unit takes a unit and a unit of a map's class and nothing else, Gizmo every gizmo, a map's gizmo class only its own, Item the item, Destructable the destructable, and Actor all of them, each in the grid's order; the holder is never one.</summary>
        public static void OtherKinds(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var plain = UnitAt(world, 50.0f, 0.0f);
            var logger = LoggerAt(world, 60.0f, 0.0f);
            var itemHandle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(itemHandle, 70.0f, 0.0f);
            world.PlacedItems.Add(itemHandle);
            var item = Item.Of(itemHandle)!;
            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(handle, 80.0f, 0.0f);
            world.PlacedDestructables.Add(handle);
            var barrel = Destructable.Of(handle)!;
            var well = Gizmo.Create(new Well(), 90.0f, 0.0f, Game.Instance);
            var shadow = Gizmo.Create(new ShadowWell(), 95.0f, 0.0f, Game.Instance);
            var units = a.Interact(new Census<Unit>(), Game.Instance);
            var gizmos = a.Interact(new Census<Gizmo>(), Game.Instance);
            var wells = a.Interact(new Census<Well>(), Game.Instance);
            var items = a.Interact(new Census<Item>(), Game.Instance);
            var destructables = a.Interact(new Census<Destructable>(), Game.Instance);
            var actors = a.Interact(new Census<Actor>(), Game.Instance);
            Steps(world, 1);
            Expect.Equal(2, units.Met.Count, "Unit takes the two units");
            Expect.Same(plain, units.Met[0], "the plain one first, in the grid's order");
            Expect.Same(logger, units.Met[1], "then the one of a map's class");
            Expect.Equal(2, gizmos.Met.Count, "Gizmo takes both gizmos");
            Expect.Same(well, gizmos.Met[0], "the well");
            Expect.Same(shadow, gizmos.Met[1], "and the other class of gizmo");
            Expect.Equal(1, wells.Met.Count, "Well takes only the well");
            Expect.Same(well, wells.Met[0], "which is it");
            Expect.Equal(1, items.Met.Count, "Item takes the item");
            Expect.Same(item, items.Met[0], "which is it");
            Expect.Equal(1, destructables.Met.Count, "Destructable takes the destructable");
            Expect.Same(barrel, destructables.Met[0], "which is it");
            Expect.Equal(6, actors.Met.Count, "Actor takes all six, and not the holder");
        }

        /// <summary>50 auras of a half-second tick (16 steps) started in one step take turns over its steps for their first checks: the first a whole tick later, the next a step sooner, and so on round, so each step of the tick has three or four; each then checks a tick after its first. One started alone later first checks a whole tick after it starts; eight started one a step keep that spread, each a whole tick after its start; and a lone aura after an install checks a whole tick after it starts.</summary>
        public static void SpreadFirstChecks(FakeWorld world)
        {
            Fresh();
            var auras = new List<FireAura>();
            for (var i = 0; i < 50; i++)
            {
                var holder = UnitAt(world, i * 20.0f - 500.0f, 0.0f);
                auras.Add(holder.Interact(new FireAura(), Game.Instance));
            }

            var start = Tracking.StepNow;
            Steps(world, 33);
            var perStep = new int[17];
            for (var i = 0; i < 50; i++)
            {
                // Each pair ticks at a check, so the steps after the first that differ from it are the next check's.
                var steps = auras[i].TickSteps;
                var first = steps[0] - start;
                var next = -1;
                for (var k = 1; k < steps.Count && next < 0; k++)
                {
                    if (steps[k] != steps[0])
                    {
                        next = steps[k];
                    }
                }

                Expect.Equal(16 - i % 16, first, "aura " + i + "'s first check");
                Expect.Equal(16, next - steps[0], "aura " + i + " checks a tick after its first");
                perStep[first]++;
            }

            for (var step = 1; step <= 16; step++)
            {
                Expect.True(perStep[step] == 3 || perStep[step] == 4, "three or four first checks at step " + step);
            }

            var late = UnitAt(world, 600.0f, 0.0f);
            var alone = late.Interact(new FireAura(), Game.Instance);
            var at = Tracking.StepNow;
            Steps(world, 16);
            Expect.True(alone.TickSteps.Count >= 1, "one started alone later checks within a tick");
            Expect.Equal(at + 16, alone.TickSteps[0], "a whole tick after it starts, as the first of its step");
            var spaced = new List<FireAura>();
            var starts = new List<int>();
            for (var i = 0; i < 8; i++)
            {
                spaced.Add(UnitAt(world, i * 20.0f + 700.0f, 300.0f).Interact(new FireAura(), Game.Instance));
                starts.Add(Tracking.StepNow);
                Steps(world, 1);
            }

            Steps(world, 16);
            for (var i = 0; i < 8; i++)
            {
                Expect.Equal(starts[i] + 16, spaced[i].TickSteps[0], "one started a step after another checks a step after it, a whole tick after its start");
            }
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var lone = a.Interact(new FireAura(), Game.Instance);
            var from = Tracking.StepNow;
            Steps(world, 16);
            Expect.True(lone.TickSteps.Count >= 1, "a lone aura after an install checks in its first tick");
            Expect.Equal(from + 16, lone.TickSteps[0], "a whole tick after it starts");
            Expect.Equal(from + 16, lone.TickSteps[lone.TickSteps.Count - 1], "and only then");
        }

        /// <summary>
        /// With no slack the check's reach is its range, 300, and its box the cells within 300 of a holder at (0, 0), on a
        /// cell's corner: things on the diagonals just within range, on cells' edges and on a cell's corner meet; things just
        /// past range, and things in the box's corner cells beyond range, do not; and the order is the grid's.
        /// </summary>
        public static void CornerCells(FakeWorld world)
        {
            Fresh();
            Interactions.Slack = 0.0f;
            var a = UnitAt(world, 0.0f, 0.0f);
            var names = new Dictionary<Unit, string>();
            var spots = new float[]
            {
                212.0f, 212.0f, -212.0f, 212.0f, 212.0f, -212.0f, -212.0f, -212.0f,
                212.5f, 212.5f, -212.5f, -212.5f,
                300.0f, 0.0f, 0.0f, -300.0f, 256.0f, 128.0f, -128.0f, 256.0f,
                290.0f, 290.0f, -290.0f, 290.0f, 380.0f, 380.0f,
            };
            var tags = new string[]
            {
                "ne", "nw", "se", "sw", "ne past", "sw past", "e", "s", "e up", "n left", "ne far", "nw far", "ne corner",
            };
            for (var i = 0; i < spots.Length; i += 2)
            {
                names[UnitAt(world, spots[i], spots[i + 1])] = tags[i / 2];
            }

            var aura = new FireAura();
            aura.Tag = unit => names[unit];
            a.Interact(aura, Game.Instance);
            Steps(world, 16);
            ExpectLog(new string[]
            {
                "meet s", "tick s", "meet sw", "tick sw", "meet se", "tick se", "meet e", "tick e",
                "meet nw", "tick nw", "meet ne", "tick ne", "meet e up", "tick e up", "meet n left", "tick n left",
            }, "the things within range meet, in the grid's order, and none past it");
        }
    }
}
