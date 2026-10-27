using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to the interactions package, on the default settings: a step of 1/32 second and a cell check of 0.1 second.</summary>
    public static class AuditInteractionCases
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

        /// <summary>Makes units in a row far from where a case works, so the cell checks take more than a step to come round to its units.</summary>
        private static void Crowd(FakeWorld world, int count)
        {
            for (var i = 0; i < count; i++)
            {
                UnitAt(world, (float)(i * 10), 0.0f);
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

        /// <summary>Checks that the test log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
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

        /// <summary>A holder and its other moved far together in one jump (Mass Teleport, a map's teleport) are in range at every check: the pair does not part and meet again, and ticks on. The moved other is found at its new place within a cell check.</summary>
        public static void TeleportTogetherStaysPaired(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            Expect.Equal(1, aura.Pairs.Count, "a and b have met");
            var pair = aura.Pairs[0];
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(0, CountOf("part b"), "b, in range at every check, does not part");
            Expect.Equal(0, CountOf("meet b"), "nor meet again");
            Expect.Equal(8, CountOf("tick b"), "it ticks at every step");
            Expect.Same(pair, aura.Pairs[0], "the pair is the same");
            Expect.Near(900.0f, pair.OtherX, "its reads are of the new place");
            Expect.Equal(1, Find.InRange<Unit>(900.0f, 800.0f, 50.0f).Count, "within a cell check Find finds b at its new place");
        }

        /// <summary>A pair stopped (the two meet again only once the other has been out of range), then holder and other moved far together: the other was never out of range, so it does not meet again.</summary>
        public static void TeleportTogetherKeepsStop(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            aura.Pairs[0].Stop();
            Steps(world, 4);
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(0, CountOf("meet b"), "the stopped other, never out of range, does not meet again");
            world.MoveUnit(b.Handle, -700.0f, -800.0f);
            Steps(world, 8);
            world.MoveUnit(b.Handle, 900.0f, 800.0f);
            Steps(world, 8);
            Expect.Equal(1, CountOf("meet b"), "out of range once, it meets again");
        }

        /// <summary>A holder moved far away alone parts with its other at its next check, once; the other's direct read does not keep a pair that is apart.</summary>
        public static void TeleportApartParts(FakeWorld world)
        {
            Fresh();
            Crowd(world, 30);
            var a = UnitAt(world, -800.0f, -800.0f);
            var b = UnitAt(world, -700.0f, -800.0f);
            var aura = new FireAura(300.0f, 0.03125f);
            aura.Tag = unit => unit == b ? "b" : "?";
            a.Interact(aura);
            Steps(world, 4);
            TestLog.Lines.Clear();
            world.MoveUnit(a.Handle, 800.0f, 800.0f);
            Steps(world, 1);
            Expect.Equal(1, CountOf("part b"), "b parts at the next check");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            Steps(world, 8);
            Expect.Equal(1, CountOf("part b"), "once");
            Expect.Equal(0, CountOf("meet b"), "and does not meet again");
        }

        /// <summary>The guide's FireAura burns an enemy; the unit changes sides (Charm, Possession, a new owner) and is no longer the holder's enemy: at the next tick Accepts says no, the pair parts and the burning stops. Back with the enemy, it is accepted at the next tick and burns again.</summary>
        public static void GuideFireAuraSparesChangedOwner(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), 1);
            world.MoveUnit(handle, 100.0f, 0.0f);
            var b = Unit.Of(handle)!;
            var aura = new GuideFireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.True(aura.RecheckAccepts, "Accepts is asked again unless the interaction says not");
            a.Interact(aura);
            Steps(world, 32);
            ExpectLog(new string[] { "meet b", "burn b", "burn b" }, "b, an enemy, burns");
            Expect.Equal(2, world.Dealt.Count, "twice");
            b.Player = a.Player;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "part b" }, "charmed: the pair parts at the next tick");
            Steps(world, 32);
            Expect.Equal(2, world.Dealt.Count, "and b is burnt no more");
            Expect.Equal(0, aura.Pairs.Count, "no pair");
            b.Player = Player.Get(1);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "part b", "meet b", "burn b" }, "an enemy again, b meets again at the next tick");
            Expect.Equal(3, world.Dealt.Count, "and burns");
        }

        /// <summary>With RecheckAccepts off, Accepts is asked only before a meeting, as before decision 73: the charmed unit burns on.</summary>
        public static void RecheckOffKeepsPair(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var handle = world.NewUnit(ObjectTypes.FourCC("hfoo"), 1);
            world.MoveUnit(handle, 100.0f, 0.0f);
            var b = Unit.Of(handle)!;
            var aura = new GuideFireAura(false);
            aura.Tag = unit => unit == b ? "b" : "?";
            Expect.True(!aura.RecheckAccepts, "turned off");
            a.Interact(aura);
            Steps(world, 16);
            b.Player = a.Player;
            Steps(world, 32);
            ExpectLog(new string[] { "meet b", "burn b", "burn b", "burn b" }, "the pair is kept and burns on");
        }

        /// <summary>A pair that Accepts refuses at a tick parts (OnPart once), and the other is not asked again in that check; it is asked again at the next tick, as any other in range, and meets again once accepted. Accepts runs with the interaction as the current owner, and a paused pair is asked too, as it still parts.</summary>
        public static void RecheckRefusalPartsAndMeetsAgain(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            var accept = true;
            aura.Accepting = unit => accept;
            a.Interact(aura);
            Steps(world, 16);
            Expect.Equal(1, aura.Asked, "asked before the meeting");
            Steps(world, 16);
            Expect.Equal(2, aura.Asked, "and again before the next tick");
            ExpectLog(new string[] { "meet b", "tick b", "tick b" }, "accepted, it ticks");
            aura.AskedAs = null;
            accept = false;
            Steps(world, 16);
            Expect.Equal(3, aura.Asked, "refused once in the check, not asked again in it");
            Expect.Same(aura, aura.AskedAs, "with the interaction as the current owner");
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b" }, "the refused pair parts, with no tick");
            Steps(world, 16);
            Expect.Equal(4, aura.Asked, "asked again at the next tick");
            Expect.Equal(4, TestLog.Lines.Count, "and refused, no meeting");
            accept = true;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b", "meet b", "tick b" }, "accepted again, it meets again");
            aura.Pairs[0].Pause();
            accept = false;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "tick b", "part b", "meet b", "tick b", "part b" }, "a paused pair refused parts too");
        }

        /// <summary>An Accepts that throws at a tick is reported and refuses, so the pair parts once; asked again at the next tick it throws again and no meeting comes.</summary>
        public static void RecheckThrowingAcceptsRefuses(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : "?";
            var throws = false;
            aura.Accepting = unit =>
            {
                if (throws)
                {
                    throw new System.InvalidOperationException("no");
                }

                return true;
            };
            a.Interact(aura);
            Steps(world, 16);
            throws = true;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "the Accepts that threw refused: the pair parted");
            Expect.True(Logged(world, "FireAura.Accepts failed"), "the error is reported");
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "part b" }, "asked again, it threw again: no meeting");
            Expect.Equal(3, aura.Asked, "asked once at each check");
            Expect.Same(Runtime.Root, Owner.Current, "the root is the current owner after the step");
        }

        /// <summary>OnPart runs once per OnMeet when Accepts, asked again at a tick, ends a pair itself, ends its other, or ends the interaction: each pair parts once, and the check stops when the interaction ends.</summary>
        public static void RecheckPartsOncePerMeet(FakeWorld world)
        {
            Fresh();
            var a = UnitAt(world, 0.0f, 0.0f);
            var b = UnitAt(world, 100.0f, 0.0f);
            var c = UnitAt(world, 0.0f, 100.0f);
            var d = UnitAt(world, 200.0f, 0.0f);
            var aura = new FireAura();
            aura.Tag = unit => unit == b ? "b" : unit == c ? "c" : unit == d ? "d" : "?";
            var round = 0;
            aura.Accepting = unit =>
            {
                if (round == 1 && unit == b)
                {
                    // Ends b's own pair from inside Accepts, then refuses: one part.
                    aura.Pairs[0].End();
                    return false;
                }

                if (round == 1 && unit == c)
                {
                    // Ends the other: its pairs part as it ends, and Accepts says yes, too late.
                    c.Remove();
                    return true;
                }

                if (round == 2)
                {
                    aura.End();
                    return false;
                }

                return true;
            };
            a.Interact(aura);
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "meet d", "tick d" }, "three pairs");
            round = 1;
            Steps(world, 16);
            ExpectLog(new string[] { "meet b", "tick b", "meet c", "tick c", "meet d", "tick d", "part b", "part c", "tick d" }, "b's pair ended in Accepts parts once; c ended parts once; d ticks");
            round = 2;
            Steps(world, 16);
            Expect.True(aura.HasEnded, "the interaction ended in Accepts");
            Expect.Equal(1, CountOf("part d"), "d parts once");
            Expect.Equal(1, CountOf("part b"), "b parted once in all");
            Expect.Equal(1, CountOf("part c"), "c parted once in all");
            Expect.Equal(CountOf("meet b") + CountOf("meet c") + CountOf("meet d"), CountOf("part b") + CountOf("part c") + CountOf("part d"), "OnPart once per OnMeet");
        }

        /// <summary>Units removed from the cell checks before, at and after the place the round has got to, and the last one: each unit left is read once in what is left of the round, those not yet read first, before any is read again.</summary>
        public static void CellCheckRemovalKeepsRound(FakeWorld world)
        {
            var fake = Fresh();
            var units = new List<Unit>();
            for (var i = 0; i < 32; i++)
            {
                units.Add(UnitAt(world, 0.0f, 0.0f));
            }

            Tracking.Need(Tracking.UnitKind);
            fake.UnitReads.Clear();
            Steps(world, 1);
            Expect.Equal(10, fake.UnitReads.Count, "32 units: the first step read 10, the first 10");
            var unread = new List<Unit>();
            var read = new List<Unit>();
            for (var i = 0; i < 32; i++)
            {
                if (i == 3 || i == 10 || i == 20 || i == 31)
                {
                    continue;
                }

                if (i < 10)
                {
                    read.Add(units[i]);
                }
                else
                {
                    unread.Add(units[i]);
                }
            }

            units[3].Remove();
            units[10].Remove();
            units[20].Remove();
            units[31].Remove();
            units[2].Remove();
            read.Remove(units[2]);
            units[30].Remove();
            unread.Remove(units[30]);
            fake.UnitReads.Clear();
            Steps(world, 4);
            var left = unread.Count + read.Count;
            Expect.Equal(26, left, "26 units are left");
            Expect.True(fake.UnitReads.Count >= left, "four steps read the round");
            for (var i = 0; i < left; i++)
            {
                var unit = Unit.Of(fake.UnitReads[i])!;
                var first = i < unread.Count;
                Expect.True(first ? unread.Contains(unit) : read.Contains(unit), "read " + i + " is of a unit " + (first ? "not yet read" : "read before"));
                Expect.True(fake.UnitReads.IndexOf(fake.UnitReads[i]) == i, "read " + i + " is the unit's first");
            }
        }

        /// <summary>A destructable that holds an interaction pairing with units is tracked alone: the destructables of the map are not taken in, until something asks for destructables. A Find of destructables inside Local.For lists the holder once; the first Find outside takes in the rest and keeps the holder's entry.</summary>
        public static void DestructableHolderTrackedAlone(FakeWorld world)
        {
            Fresh();
            for (var i = 0; i < 50; i++)
            {
                var other = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
                world.PlaceDestructable(other, (float)(i * 10), 500.0f);
                world.PlacedDestructables.Add(other);
            }

            var handle = world.NewDestructable(ObjectTypes.FourCC("dfoo"));
            world.PlaceDestructable(handle, 0.0f, 0.0f);
            world.PlacedDestructables.Add(handle);
            var barrel = Destructable.Of(handle)!;
            var b = UnitAt(world, 100.0f, 0.0f);
            var shine = new Shine();
            shine.Tag = unit => unit == b ? "b" : "?";
            barrel.Interact(shine);
            Expect.Equal(2, Tracking.Count, "the barrel and the unit are tracked");
            Expect.Equal(1, KnownDestructables.Live.Count, "no other destructable is taken in");
            Expect.True(!Tracking.Asked(Tracking.DestructableKind), "destructables are not asked for");
            Steps(world, 1);
            ExpectLog(new string[] { "meet b" }, "the barrel meets b");
            var local = new List<Destructable>();
            Local.For(Player.Get(0), () => { local = Find.InRange<Destructable>(0.0f, 0.0f, 100.0f); });
            Expect.Equal(1, local.Count, "inside Local.For a Find lists the barrel once");
            var entry = Tracking.EntryOf(barrel);
            var found = Find.InRange<Destructable>(0.0f, 0.0f, 100.0f);
            Expect.Equal(1, found.Count, "the first Find outside finds the barrel once");
            Expect.Equal(52, Tracking.Count, "and takes in the destructables of the map");
            Expect.Same(entry, Tracking.EntryOf(barrel), "keeping the barrel's entry");
            barrel.Remove();
            ExpectLog(new string[] { "meet b", "part b" }, "the shine parts with b as the barrel goes");
            Expect.True(shine.HasEnded, "and ends");
        }

        /// <summary>An item that holds an interaction is tracked alone: the items of the map are not, and no scan looks for new ones once a second.</summary>
        public static void ItemHolderTrackedAlone(FakeWorld world)
        {
            Fresh();
            for (var i = 0; i < 3; i++)
            {
                var lying = world.NewItem(ObjectTypes.FourCC("ofoo"));
                world.MoveItem(lying, (float)(i * 10), 500.0f);
                world.PlacedItems.Add(lying);
            }

            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, 0.0f, 0.0f);
            world.PlacedItems.Add(handle);
            var orb = Item.Of(handle)!;
            var b = UnitAt(world, 100.0f, 0.0f);
            var shine = new Shine();
            shine.Tag = unit => unit == b ? "b" : "?";
            orb.Interact(shine);
            Expect.Equal(2, Tracking.Count, "the orb and the unit are tracked");
            Expect.True(!Tracking.Asked(Tracking.ItemKind), "items are not asked for");
            var dropped = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(dropped, 50.0f, 60.0f);
            world.PlacedItems.Add(dropped);
            Steps(world, 64);
            Expect.True(KnownItems.Find(dropped) == null, "no scan takes in a new ground item");
            Expect.Equal(1, KnownItems.Live.Count, "nor the items lying there");
            ExpectLog(new string[] { "meet b" }, "the orb meets b");
            world.MoveItem(handle, 900.0f, 900.0f);
            Steps(world, 4);
            ExpectLog(new string[] { "meet b", "part b" }, "moved away, it parts with b");
        }

        /// <summary>A unit that holds an interaction with gizmos is tracked alone: the units of the map are not taken in.</summary>
        public static void UnitHolderOfGizmosTrackedAlone(FakeWorld world)
        {
            Fresh();
            UnitAt(world, 500.0f, 500.0f);
            var a = UnitAt(world, 0.0f, 0.0f);
            var well = Gizmo.Create(new Well(), 100.0f, 0.0f);
            var census = a.Interact(new Census<Gizmo>());
            Expect.Equal(2, Tracking.Count, "the holder and the well are tracked");
            Expect.True(!Tracking.Asked(Tracking.UnitKind), "units are not asked for");
            Steps(world, 1);
            Expect.Equal(1, census.Met.Count, "the unit meets the well");
            Expect.Same(well, census.Met[0], "that well");
            a.Remove();
            Expect.True(census.HasEnded, "the census ends with its holder");
            Expect.Equal(1, Tracking.Count, "which is no longer tracked");
        }

        /// <summary>An item in a unit's slot has no place, also one whose pick-up Moonwell never saw, so it has no Carrier: the game says an item in a slot is not visible (seen 10-26), and the place asks that. Dropped, out of the slot and visible, it has a place at the unit's.</summary>
        public static void CarriedItemHasNoPlace(FakeWorld world)
        {
            var fake = Fresh();
            Tracking.Need(Tracking.ItemKind);
            var hero = UnitAt(world, 0.0f, 0.0f);
            var handle = world.NewItem(ObjectTypes.FourCC("ofoo"));
            world.MoveItem(handle, 40.0f, 0.0f);
            world.PutInSlot(hero.Handle, 0, handle);
            fake.Unseen.Add(handle);
            var orb = Item.Of(handle)!;
            Expect.True(orb.Carrier == null, "Moonwell never saw it picked up");
            var entry = Tracking.EntryOf(orb);
            Expect.True(entry != null, "it is tracked");
            Expect.True(!entry!.HasPlace, "and has no place");
            Steps(world, 4);
            Expect.True(!entry.HasPlace, "nor after its cell checks");
            Expect.Equal(0, Find.InRange<Item>(40.0f, 0.0f, 100.0f).Count, "a Find does not find it where it was picked up");
            world.DropItem(hero.Handle, handle);
            fake.Unseen.Remove(handle);
            Steps(world, 4);
            Expect.True(entry.HasPlace, "dropped, it has a place");
            Expect.Equal(1, Find.InRange<Item>(0.0f, 0.0f, 10.0f).Count, "at the unit's");
        }

        /// <summary>In a gizmo's OnLeave the gizmo has no place: a Find from it does not see it, though it is still in its cell, and a gizmo beside it is found; it leaves the cell after.</summary>
        public static void GizmoLeaveHasNoPlace(FakeWorld world)
        {
            Fresh();
            var well = new Well();
            var beside = Gizmo.Create(new Well(), 20.0f, 10.0f);
            var found = new List<Well>();
            var cell = -2;
            well.Leaving = () =>
            {
                found = Find.InRange<Well>(10.0f, 10.0f, 50.0f);
                cell = well.entry!.Cell;
            };
            Gizmo.Create(well, 10.0f, 10.0f);
            well.End();
            Expect.Equal(1, found.Count, "a Find from OnLeave finds only the gizmo beside it");
            Expect.Same(beside, found[0], "that one");
            Expect.True(cell >= 0, "though the gizmo is still in its cell");
            Expect.Equal(-1, well.entry!.Cell, "and leaves it after");
        }
    }
}
