namespace Moonwell.Analyzers.Tests;

/// <summary>MW0040 for [MachineLocal] on code that runs on every machine, where the attribute cannot make it local.</summary>
public class MachineLocalTests
{
    [Fact]
    public void machine_local_on_code_run_on_every_machine_is_mw0040()
    {
        var found = MapCode.CheckFile("""
            using System;
            using Moonwell;

            namespace Map
            {
                public sealed class Hud : PlayerComponent
                {
                    public static float LastX;

                    [MachineLocal]
                    public override void OnGameStart()
                    {
                        LastX = Natives.GetCameraTargetPositionX();
                    }
                }

                public interface IPanel
                {
                    void Draw();
                }

                public sealed class Panel : IPanel
                {
                    [MachineLocal]
                    public void Draw()
                    {
                    }
                }

                public static class Use
                {
                    private static float last;

                    public static event Action? Changed;

                    public static void Run(Player p)
                    {
                        Timer.Every(1f, [MachineLocal] () => { last = Natives.GetCameraTargetPositionX(); });
                        Changed += [MachineLocal] () => { last = Natives.GetCameraTargetPositionY(); };
                        Local.For(p, [MachineLocal] () => { last = Natives.GetCameraTargetPositionX(); });
                        Action later = [MachineLocal] () => { };
                    }
                }
            }
            """);

        Assert.Equal(new[]
        {
            "MW0040: OnGameStart",
            "MW0040: Draw",
            "MW0040: [MachineLocal] () => { last = Natives.GetCameraTargetPositionX(); }",
            "MW0040: [MachineLocal] () => { last = Natives.GetCameraTargetPositionY(); }",
        }, found);
    }

    [Fact]
    public void machine_local_where_the_map_marks_what_it_overrides_is_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;

            namespace Map
            {
                public class Base
                {
                    [MachineLocal]
                    public virtual float Read() => Natives.GetCameraTargetPositionX();
                }

                public sealed class Derived : Base
                {
                    [MachineLocal]
                    public override float Read() => Natives.GetCameraTargetPositionY();
                }

                public interface IPanel
                {
                    [MachineLocal]
                    void Draw();
                }

                public sealed class Panel : IPanel
                {
                    [MachineLocal]
                    public void Draw() => Natives.GetCameraTargetPositionX();
                }
            }
            """));
    }

    [Fact]
    public void an_override_of_a_machine_local_method_without_it_is_followed()
    {
        var found = MapCode.Check(
            """
            Base b = new Derived();
            Local.For(p, () => b.Show());
            """,
            """
                public class Base
                {
                    [MachineLocal]
                    public virtual void Show() { }
                }

                public sealed class Derived : Base
                {
                    public override void Show() => Natives.KillUnit(Natives.GetTriggerUnit());
                }
            """);

        Assert.Equal(new[] { "MW0001: b.Show()" }, found);
    }

    [Fact]
    public void machine_local_on_an_override_says_what_to_write()
    {
        Assert.Equal(new[]
        {
            "'Hud.OnGameStart' is [MachineLocal], but it overrides MapComponent.OnGameStart, which is called on every machine; put its machine-local code in Local.For(player, ...)",
        }, MapCode.MessagesOfFile("""
            using Moonwell;

            namespace Map
            {
                public sealed class Hud : PlayerComponent
                {
                    [MachineLocal]
                    public override void OnGameStart() { }
                }
            }
            """));
    }
}
