using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>Cases that check which hooks a class overrides, and which game events the runtime listens for.</summary>
    public static class HookCases
    {
        /// <summary>A class's mask has the bit of each hook it overrides, and no others.</summary>
        public static void Overrides(FakeWorld world)
        {
            var table = Table();
            Expect.Equal(Hook.OnDamaged, Hooks.MaskOf(new HookSub(), table), "HookSub overrides OnDamaged");
            Expect.Equal(Hook.OnDamaged, Hooks.MaskOf(new HookSubSub(), table), "HookSubSub inherits the override");
            Expect.Equal(0, Hooks.MaskOf(new HookOther(), table), "HookOther overrides nothing");
            HookBase seen = new HookSub();
            Expect.Equal(Hook.OnDamaged, Hooks.MaskOf(seen, table), "HookSub seen through a HookBase variable");
        }

        /// <summary>A class's mask is worked out once: two objects of one class give the same mask.</summary>
        public static void Cached(FakeWorld world)
        {
            var table = Table();
            var first = Hooks.MaskOf(new HookSub(), table);
            var second = Hooks.MaskOf(new HookSub(), table);
            Expect.Equal(Hook.OnDamaged, first, "the first HookSub");
            Expect.Equal(first, second, "the second HookSub gives the same mask");
        }

        /// <summary>Each game event is listened for once, however many bits ask for it, and only when a bit needs it.</summary>
        public static void NeedListensOnce(FakeWorld world)
        {
            // The runtime already listens for pick-ups and drops from its start; this case counts what Need adds.
            Events.Reset();
            world.Listening.Clear();
            Events.Need(Hook.OnDamaged | Hook.OnUnitDamaged);
            Events.Need(Hook.OnDamaged);
            Expect.Equal(1, world.Listening.Count, "one event is listened for");
            Expect.Equal((int)GameEvent.Damage, (int)world.Listening[0], "the event is Damage");

            Events.Need(Hook.OnKill);
            Expect.Equal(2, world.Listening.Count, "OnKill adds one event");
            Expect.Equal((int)GameEvent.Death, (int)world.Listening[1], "the added event is Death");

            Events.Need(Hook.OnGameStart);
            Expect.Equal(2, world.Listening.Count, "OnGameStart adds no event");

            Expect.Equal(true, Events.Wants(Hook.OnUnitDamaged), "OnUnitDamaged is wanted");
            Expect.Equal(false, Events.Wants(Hook.OnCast), "OnCast is not wanted");
        }

        /// <summary>With a damage listener set, a damage reaction calls it, and the world is not asked to deliver damage itself.</summary>
        public static void DamageListener(FakeWorld world)
        {
            var calls = 0;
            Events.DamageListener = () => calls++;
            ObjectTypes.Unit(ObjectTypes.FourCC("h000"), () => new Dummy());
            Expect.Equal(1, calls, "the listener is called once");
            Expect.True(!world.Listening.Contains(GameEvent.Damage), "the world does not listen for Damage");
        }

        /// <summary>The seen hook gets the first object of each class, once per class.</summary>
        public static void SeenOncePerClass(FakeWorld world)
        {
            var seen = new System.Collections.Generic.List<string>();
            Hooks.Seen = obj => seen.Add(Names.NameOf(obj));
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("ofoo")))!;
            unit.Add(new DummyComp());
            unit.Add(new DummyComp());
            var comps = 0;
            for (var i = 0; i < seen.Count; i++)
            {
                if (seen[i] == "DummyComp")
                {
                    comps++;
                }
            }

            Expect.Equal(1, comps, "DummyComp is seen once");
        }

        /// <summary>A fresh runtime has neither hook set.</summary>
        public static void ResetClears(FakeWorld world)
        {
            Events.DamageListener = () => { };
            Hooks.Seen = _ => { };
            FakeWorld.Install();
            Expect.True(Events.DamageListener == null, "the damage listener is cleared");
            Expect.True(Hooks.Seen == null, "the seen hook is cleared");
        }

        /// <summary>TakenIn sees each unit, item and destructable taken in, and Ending sees each one before its OnLeave runs; a hook that throws is reported, and the object still ends.</summary>
        public static void TakenInAndEnding(FakeWorld world)
        {
            TestLog.Lines.Clear();
            var seen = new System.Collections.Generic.List<Widget>();
            Hooks.TakenIn = widget =>
            {
                seen.Add(widget);
                TestLog.Lines.Add("in " + Kind(widget));
            };
            Hooks.Ending = widget => TestLog.Lines.Add("ending " + Kind(widget));
            ObjectTypes.Unit(ObjectTypes.FourCC("h004"), () => new LeaveLogger());
            var unit = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h004")))!;
            var item = Item.Of(world.NewItem(ObjectTypes.FourCC("ratc")))!;
            var destructable = Destructable.Of(world.NewDestructable(ObjectTypes.FourCC("dfoo")))!;
            Expect.Equal(3, seen.Count, "the objects taken in");
            Expect.Same(unit, seen[0], "the unit");
            Expect.Same(item, seen[1], "the item");
            Expect.Same(destructable, seen[2], "the destructable");
            Expect.Same(unit, Unit.Of(unit.Handle), "the same unit again");
            Expect.Equal(3, seen.Count, "an object already held is not taken in again");
            ExpectLog(new string[] { "in unit", "in item", "in destructable" }, "the take-ins");

            TestLog.Lines.Clear();
            unit.Remove();
            item.Remove();
            destructable.Remove();
            ExpectLog(new string[] { "ending unit", "leave", "ending item", "ending destructable" }, "the endings");

            Hooks.Ending = widget => { throw new System.InvalidOperationException("boom"); };
            TestLog.Lines.Clear();
            var second = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h004")))!;
            second.Remove();
            Expect.True(second.HasEnded, "the unit ended");
            ExpectLog(new string[] { "in unit", "leave" }, "OnLeave still ran");
            Expect.True(world.Logged.Contains("Moonwell: Moonwell.Hooks failed: boom"), "the logged error");

            Hooks.TakenIn = widget => { throw new System.InvalidOperationException("bang"); };
            var third = Unit.Of(world.NewUnit(ObjectTypes.FourCC("h004")))!;
            Expect.True(!third.HasEnded, "the unit is taken in");
            Expect.True(world.Logged.Contains("Moonwell: Moonwell.Hooks failed: bang"), "the logged error of the take-in");
        }

        /// <summary>A fresh runtime has neither widget hook set.</summary>
        public static void WidgetHooksReset(FakeWorld world)
        {
            Hooks.TakenIn = _ => { };
            Hooks.Ending = _ => { };
            FakeWorld.Install();
            Expect.True(Hooks.TakenIn == null, "the taken-in hook is cleared");
            Expect.True(Hooks.Ending == null, "the ending hook is cleared");
        }

        /// <summary>The name of a widget's kind, for the log.</summary>
        private static string Kind(Widget widget)
        {
            if (widget is Unit)
            {
                return "unit";
            }

            return widget is Item ? "item" : "destructable";
        }

        /// <summary>Checks that the log is exactly the expected lines, in order.</summary>
        private static void ExpectLog(string[] expected, string what)
        {
            Expect.Equal(expected.Length, TestLog.Lines.Count, what + ": line count");
            for (var i = 0; i < expected.Length; i++)
            {
                Expect.Equal(expected[i], TestLog.Lines[i], what + ": line " + i);
            }
        }

        /// <summary>The table of the test classes: OnDamaged and OnKill, declared by HookBase.</summary>
        private static HookTable Table()
        {
            return new HookTable(typeof(HookBase), new[] { "OnDamaged", "OnKill" }, new[] { Hook.OnDamaged, Hook.OnKill });
        }
    }
}
