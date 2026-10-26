namespace Moonwell.Analyzers.Tests;

/// <summary>The sync rules (MW0001, MW0002, MW0004, MW0005) on forms of code the other tests do not write.</summary>
public class SyncFormsTests
{
    [Fact]
    public void incrementing_or_decrementing_gold_inside_is_rejected()
    {
        var found = MapCode.Check("""
            Local.For(p, () =>
            {
                p.Gold++;
                p.Lumber--;
            });
            """);

        Assert.Equal(new[] { "MW0001: p.Gold", "MW0001: p.Lumber" }, found);
    }

    [Fact]
    public void a_named_action_argument_is_a_local_block()
    {
        var found = MapCode.Check("""
            Local.For(p, action: () => Natives.KillUnit(u));
            Local.For(action: () => Natives.RemoveUnit(u), player: p);
            """);

        Assert.Equal(new[] { "MW0001: Natives.KillUnit(u)", "MW0001: Natives.RemoveUnit(u)" }, found);
    }

    [Fact]
    public void a_machine_local_lambda_or_local_function_is_checked_as_a_block()
    {
        var found = MapCode.Check("""
            var hud = [MachineLocal] () =>
            {
                var x = Natives.GetCameraTargetPositionX();
                Natives.KillUnit(u);
            };

            [MachineLocal]
            void Show()
            {
                var y = Natives.GetCameraTargetPositionY();
                Natives.RemoveUnit(u);
            }

            Local.For(p, () => Show());
            """);

        Assert.Equal(new[] { "MW0001: Natives.KillUnit(u)", "MW0001: Natives.RemoveUnit(u)" }, found);
    }

    [Fact]
    public void arrays_of_number_types_the_game_cannot_carry_are_rejected()
    {
        var found = MapCode.Check(
            """
            double[] rates = new double[2];
            long[] totals = { 1, 2 };
            """,
            """
                private static double[] Rates() => new double[0];

                private static void Count(long[] counts) { }
            """);

        // A method is found once, by its name, for a number type in its return type or a parameter.
        Assert.Equal(new[] { "MW0005: rates", "MW0005: totals", "MW0005: Rates", "MW0005: Count" }, found);
    }

    [Fact]
    public void looping_over_a_class_derived_from_a_dictionary_or_hash_set_is_rejected()
    {
        var found = MapCode.Check(
            """
            var registry = new Registry();
            var marks = new Marks();
            foreach (var pair in registry) { }
            foreach (var mark in marks) { }
            """,
            """
                private sealed class Registry : Dictionary<unit, int> { }

                private sealed class Marks : HashSet<int> { }
            """);

        Assert.Equal(new[] { "MW0004: registry", "MW0004: marks" }, found);
    }

    [Fact]
    public void name_read_in_a_static_constructor_is_mw0002()
    {
        // A pattern reads Name through its input, as this.Name reads it; only an instance constructor of a declared class
        // may read the declared name.
        var found = MapCode.CheckFile("""
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public static Unit? Probe;

                    public static bool Guarded;

                    static Mage()
                    {
                        Guarded = Probe is Unit { Name: "Guard" };
                    }

                    public Mage()
                    {
                        Tooltip = this is { Name: "Mage" } ? "a mage" : "another";
                    }
                }
            }
            """);

        Assert.Equal(new[] { "MW0002: Name" }, found);
    }
}
