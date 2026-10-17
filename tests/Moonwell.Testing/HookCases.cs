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

        /// <summary>The table of the test classes: OnDamaged and OnKill, declared by HookBase.</summary>
        private static HookTable Table()
        {
            return new HookTable(typeof(HookBase), new[] { "OnDamaged", "OnKill" }, new[] { Hook.OnDamaged, Hook.OnKill });
        }
    }
}
