namespace Moonwell.Analyzers.Tests;

/// <summary>MW0001 and MW0002: what may run inside Local.For, and what may run only there.</summary>
public class LocalTests
{
    [Fact]
    public void natives_that_create_or_change_are_rejected_inside()
    {
        var found = MapCode.Check("""
            Local.For(p, () =>
            {
                Natives.CreateUnit(p.Handle, 0, 0f, 0f, 0f);
                Natives.KillUnit(u);
                Natives.GetRandomInt(0, 9);
                Natives.DisplayTextToPlayer(p.Handle, 0f, 0f, "hi");
                Natives.SetCameraPosition(0f, 0f);
                Natives.GetUnitX(u);
            });
            """);

        Assert.Equal(new[]
        {
            "MW0001: Natives.CreateUnit(p.Handle, 0, 0f, 0f, 0f)",
            "MW0001: Natives.KillUnit(u)",
            "MW0001: Natives.GetRandomInt(0, 9)",
        }, found);
    }

    [Fact]
    public void the_same_natives_are_allowed_outside()
    {
        Assert.Empty(MapCode.Check("""
            Natives.KillUnit(u);
            Local.For(Player.Get(Natives.GetRandomInt(0, 3)), () => { });
            """));
    }

    [Fact]
    public void setting_a_game_global_is_rejected_inside()
    {
        Assert.Equal(new[] { "MW0001: Natives.bj_lastCreatedUnit" },
            MapCode.Check("Local.For(p, () => { Natives.bj_lastCreatedUnit = u; var read = Natives.bj_lastCreatedUnit; });"));
    }

    [Fact]
    public void runtime_members_need_local_safe()
    {
        var found = MapCode.Check("""
            var unit = Unit.Of(u)!;
            Local.For(p, () =>
            {
                var slot = p.Slot;
                var handle = unit.Handle;
                var owner = unit.Player;
                var map = new OrderedMap<int, int>();
                map[1] = slot;
                unit.Remove();
                Timer.After(1f, () => { });
            });
            """);

        Assert.Equal(new[] { "MW0001: unit.Remove()", "MW0001: Timer.After(1f, () => { })" }, found);
    }

    [Fact]
    public void map_methods_are_followed()
    {
        var found = MapCode.Check(
            """
            Local.For(p, () => Spawn(p));
            Local.For(p, Kill);
            Local.For(p, () => Show(p));
            """,
            """
                private static void Spawn(Player p) => Make(p);
                private static void Make(Player p) => Natives.CreateUnit(p.Handle, 0, 0f, 0f, 0f);
                private static void Kill() => Natives.KillUnit(Natives.GetTriggerUnit());
                private static void Show(Player p) => Natives.DisplayTextToPlayer(p.Handle, 0f, 0f, "hi");
            """);

        Assert.Equal(new[] { "MW0001: Spawn(p)", "MW0001: Kill" }, found);
    }

    [Fact]
    public void a_followed_finding_names_its_path()
    {
        var message = MapCode.Message("Local.For(p, () => Spawn(p));", """
                private static void Spawn(Player p) => Make(p);
                private static void Make(Player p) => Natives.CreateUnit(p.Handle, 0, 0f, 0f, 0f);
            """);

        Assert.Equal("'Code.Spawn' creates a handle (through Code.Spawn > Code.Make > CreateUnit); inside Local.For it runs on one machine only, which desyncs the game", message);
    }

    [Fact]
    public void machine_local_getters_are_rejected_outside()
    {
        var found = MapCode.Check("""
            var local = Natives.GetLocalPlayer();
            var x = Natives.GetCameraTargetPositionX();
            Local.For(p, () =>
            {
                var y = Natives.GetCameraTargetPositionY();
                Natives.DisplayTextToPlayer(Natives.GetLocalPlayer(), 0f, 0f, "hi");
            });
            """);

        Assert.Equal(new[] { "MW0002: Natives.GetLocalPlayer()", "MW0002: Natives.GetCameraTargetPositionX()" }, found);
    }

    [Fact]
    public void a_machine_local_method_is_checked_as_a_block_and_called_only_inside_one()
    {
        var found = MapCode.Check(
            """
            Hud(p);
            Local.For(p, () => Hud(p));
            Local.For(p, () => Mark());
            """,
            """
                [MachineLocal]
                private static void Hud(Player p)
                {
                    var x = Natives.GetCameraTargetPositionX();
                    Natives.DisplayTextToPlayer(p.Handle, 0f, 0f, "x " + x);
                    Natives.KillUnit(Natives.GetTriggerUnit());
                }

                private static void Mark()
                {
                    var unused = 0;
                }
            """);

        Assert.Equal(new[] { "MW0002: Hud(p)", "MW0001: Natives.KillUnit(Natives.GetTriggerUnit())" }, found);
    }

    [Fact]
    public void sync_unchecked_silences_a_method_and_trusts_it_when_followed()
    {
        var found = MapCode.Check(
            """
            Local.For(p, () => Trusted());
            Unchecked(p);
            """,
            """
                [SyncUnchecked]
                private static void Trusted() => Natives.KillUnit(Natives.GetTriggerUnit());

                [SyncUnchecked]
                private static void Unchecked(Player p)
                {
                    Local.For(p, () => Natives.KillUnit(Natives.GetTriggerUnit()));
                    var local = Natives.GetLocalPlayer();
                    var id = Natives.GetHandleId(local);
                    long big = 1L;
                    foreach (var pair in new Dictionary<int, int>())
                    {
                    }
                }
            """);

        Assert.Empty(found);
    }
}
