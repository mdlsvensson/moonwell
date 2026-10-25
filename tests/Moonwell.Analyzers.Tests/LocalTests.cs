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
    public void enumerating_with_a_callback_is_rejected_inside()
    {
        // Hive thread 317486: ForGroup or ForForce in local code desyncs whatever it does (Insanity_AI).
        var found = MapCode.Check("""
            Local.For(p, () =>
            {
                Natives.ForGroup(null!, () => { });
                Natives.ForForce(null!, () => { });
                Natives.EnumItemsInRect(null!, null!, () => { });
                Natives.EnumDestructablesInRect(null!, null!, () => { });
                Natives.SelectUnit(u, true);
            });
            """);

        Assert.Equal(new[]
        {
            "MW0001: Natives.ForGroup(null!, () => { })",
            "MW0001: Natives.ForForce(null!, () => { })",
            "MW0001: Natives.EnumItemsInRect(null!, null!, () => { })",
            "MW0001: Natives.EnumDestructablesInRect(null!, null!, () => { })",
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
    public void damage_package_members_need_local_safe()
    {
        var found = MapCode.Check("""
            var unit = Unit.Of(u)!;
            Local.For(p, () =>
            {
                var amount = Hit!.Amount + Hit.Original;
                var tag = Hit.Tag;
                var spell = Hit.IsSpell;
                unit.Damage(unit, 1f);
            });
            """, "public static Hit? Hit;");

        Assert.Equal(new[] { "MW0001: unit.Damage(unit, 1f)" }, found);
    }

    [Fact]
    public void maths_package_readers_are_allowed_inside()
    {
        Assert.Empty(MapCode.Check("""
            var a = Unit.Of(u)!;
            var b = Unit.Of(u)!;
            Local.For(p, () =>
            {
                var d = a.DistanceTo(b) + a.AngleTo(b) + Maths.Lerp(0f, 1f, 0.5f) + Angles.Normalize(-90f);
                var v = Vector2.Up.Rotated(30f) + a.Position();
                var inside = new Polygon(Vector2.Zero, Vector2.Right, Vector2.Up).Contains(v);
                var hit = Raycast.Circle(Vector2.Zero, v, Vector2.One, 2f, out var first);
            });
            """));
    }

    [Fact]
    public void maths_package_moves_are_rejected_inside()
    {
        var found = MapCode.Check("""
            var unit = Unit.Of(u)!;
            Local.For(p, () =>
            {
                unit.PlaceAt(Vector2.Zero);
                unit.FaceNow(Vector2.Up);
            });
            """);

        Assert.Equal(new[] { "MW0001: unit.PlaceAt(Vector2.Zero)", "MW0001: unit.FaceNow(Vector2.Up)" }, found);
    }

    [Fact]
    public void player_reads_are_local_safe()
    {
        Assert.Empty(MapCode.Check("""
            Local.For(p, () =>
            {
                var one = Player.One;
                var enemy = Player.NeutralAggressive;
                var x = one.StartX + enemy.StartY;
            });
            """));
    }

    [Fact]
    public void who_is_playing_reads_inside()
    {
        Assert.Empty(MapCode.Check("""
            Local.For(p, () =>
            {
                var playing = Player.Playing.Count + Player.Users.Count;
                var left = p.SlotState == SlotState.Left || p.Controller == Controller.Computer;
            });
            """));
    }

    [Fact]
    public void printing_is_allowed_inside()
    {
        Assert.Empty(MapCode.Check("""
            Local.For(p, () =>
            {
                p.Print("hi");
                p.Print("hi", 5f);
                Game.Print("everyone");
            });
            """));
    }

    [Fact]
    public void reading_gold_inside_is_fine()
    {
        Assert.Empty(MapCode.Check("""
            Local.For(p, () =>
            {
                var gold = p.Gold + p.Lumber + p.FoodUsed;
            });
            """));
    }

    [Fact]
    public void setting_gold_inside_is_rejected()
    {
        var found = MapCode.Check("""
            Local.For(p, () =>
            {
                p.Gold = 1000;
                p.Lumber += 5;
            });
            """);

        Assert.Equal(new[] { "MW0001: p.Gold", "MW0001: p.Lumber" }, found);
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
    public void localised_tooltips_and_filter_states_are_machine_local()
    {
        var found = MapCode.Check("""
            var a = Natives.BlzGetItemTooltip(null!);
            var b = Natives.BlzGetItemDescription(null!);
            var c = Natives.BlzGetItemExtendedTooltip(null!);
            var d = Natives.BlzGetAbilityTooltip(0, 0);
            var e = Natives.BlzGetAbilityActivatedExtendedTooltip(0, 0);
            var f = Natives.BlzGetAbilityResearchTooltip(0, 0);
            var g = Natives.GetAllyColorFilterState();
            var h = Natives.GetCreepCampFilterState();
            """);

        Assert.Equal(new[]
        {
            "MW0002: Natives.BlzGetItemTooltip(null!)",
            "MW0002: Natives.BlzGetItemDescription(null!)",
            "MW0002: Natives.BlzGetItemExtendedTooltip(null!)",
            "MW0002: Natives.BlzGetAbilityTooltip(0, 0)",
            "MW0002: Natives.BlzGetAbilityActivatedExtendedTooltip(0, 0)",
            "MW0002: Natives.BlzGetAbilityResearchTooltip(0, 0)",
            "MW0002: Natives.GetAllyColorFilterState()",
            "MW0002: Natives.GetCreepCampFilterState()",
        }, found);
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

    [Fact]
    public void name_read_outside_local_for_is_mw0002()
    {
        var found = MapCode.Check("""
            var unit = Unit.Of(u)!;
            var hero = (Hero)Unit.Of(u)!;
            var outside = unit.Name;
            var proper = hero.ProperName;
            Local.For(p, () => { var inside = unit.Name + hero.ProperName; });
            """);

        Assert.Equal(new[] { "MW0002: unit.Name", "MW0002: hero.ProperName" }, found);
    }

    [Fact]
    public void a_name_read_outside_local_for_names_the_declared_name()
    {
        // 0.2.2's Name read the declared value outside the constructor; the message says where that is now.
        var found = MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public static class Use
                {
                    public static string Run(Unit unit, Hero hero)
                    {
                        return unit.Name + hero.Name + hero.ProperName + Natives.GetUnitName(unit.Handle);
                    }
                }
            }
            """);

        Assert.Equal(new[]
        {
            "'Unit.Name' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method; the declared name is Default.Name",
            "'Unit.Name' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method; the declared name is Default.Name",
            "'Hero.ProperName' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method",
            "'GetUnitName' gives a different answer on each machine; use it only inside Local.For or a [MachineLocal] method",
        }, found);
    }

    [Fact]
    public void name_set_outside_local_for_is_accepted()
    {
        Assert.Empty(MapCode.Check("""
            var unit = Unit.Of(u)!;
            var hero = (Hero)Unit.Of(u)!;
            unit.Name = "Guard";
            hero.ProperName = "Arthas";
            """));
    }

    [Fact]
    public void name_set_inside_local_for_is_mw0001()
    {
        Assert.Equal(new[] { "MW0001: unit.Name" }, MapCode.Check("""
            var unit = Unit.Of(u)!;
            Local.For(p, () => { unit.Name = "Guard"; });
            """));
    }

    [Fact]
    public void name_read_in_a_constructor_and_on_default_is_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using System;
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public Mage()
                    {
                        Name = "Mage";
                        Tooltip = Name + " " + this.Name + " " + base.Name;
                    }
                }

                public class Champion : Human.Units.Paladin
                {
                    public Champion()
                    {
                        Tooltip = Name;
                    }
                }

                public static class Use
                {
                    public static string Run(Unit unit, Hero hero)
                    {
                        return unit.Default.Name + hero.Default.Name + Unit.Default<Mage>().Name
                            + Hero.Default<Champion>().Name + Unit.Default<Mage>().Default.Name;
                    }
                }
            }
            """));
    }

    [Fact]
    public void proper_name_on_default_is_mw0002()
    {
        // A proper name has no declared value: only Name is exempt.
        Assert.Equal(new[] { "MW0002: hero.Default.ProperName", "MW0002: Hero.Default<Champion>().ProperName" }, MapCode.CheckFile("""
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Champion : Human.Units.Paladin
                {
                }

                public static class Use
                {
                    public static string Run(Hero hero)
                    {
                        return hero.Default.ProperName + Hero.Default<Champion>().ProperName;
                    }
                }
            }
            """));
    }

    [Fact]
    public void name_read_in_a_lambda_in_a_constructor_or_on_another_unit_is_mw0002()
    {
        Assert.Equal(new[] { "MW0002: Name", "MW0002: other.Name" }, MapCode.CheckFile("""
            using System;
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public Mage(Unit other)
                    {
                        Action later = () => Console.WriteLine(Name);
                        Tooltip = other.Name;
                    }
                }
            }
            """));
    }

    [Fact]
    public void name_on_a_local_copy_of_default_is_mw0002()
    {
        // The plan accepts this false alarm: only a read straight on Default is known to be declared.
        Assert.Equal(new[] { "MW0002: copy.Name" }, MapCode.Check("""
            var unit = Unit.Of(u)!;
            var copy = unit.Default;
            var name = copy.Name;
            """));
    }
}
