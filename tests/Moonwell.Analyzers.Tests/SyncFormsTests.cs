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

    [Fact]
    public void looping_through_a_dictionary_or_set_interface_is_rejected()
    {
        var found = MapCode.Check(
            """
            var map = new Dictionary<int, int>();
            IReadOnlyDictionary<int, int> view = map;
            foreach (var pair in view) { }
            var set = new HashSet<unit>();
            IReadOnlyCollection<unit> a = set;
            ISet<unit> b = set;
            ICollection<unit> c = set;
            foreach (var x in b) { }
            var first = Kills.Keys.First();
            foreach (var k in Kills.Keys) { }
            var lookup = view.TryGetValue(1, out var one) && view.ContainsKey(2) && view[1] > 0 && b.Contains(u) && view.Count > 0;
            """,
            """
                private static readonly Dictionary<unit, int> kills = new();

                public static IReadOnlyDictionary<unit, int> Kills => kills;

                private static int Sum(IDictionary<int, int> d)
                {
                    var s = 0;
                    foreach (var kv in d) { s += kv.Value; }
                    return s;
                }
            """);

        Assert.Equal(new[]
        {
            "MW0004: view",
            "MW0004: set",
            "MW0004: set",
            "MW0004: b",
            "MW0004: Kills.Keys",
            "MW0004: Kills.Keys",
            "MW0004: d",
        }, found);
    }

    [Fact]
    public void copying_a_dictionary_or_set_out_is_rejected()
    {
        var found = MapCode.Check("""
            var array = new int[4];
            var set = new HashSet<int>();
            set.CopyTo(array);
            var map = new Dictionary<int, int>();
            map.Keys.CopyTo(array, 0);
            ISet<int> view = set;
            view.CopyTo(array, 0);
            """);

        Assert.Equal(new[] { "MW0004: set.CopyTo(array)", "MW0004: map.Keys.CopyTo(array, 0)", "MW0004: view.CopyTo(array, 0)" }, found);
    }

    [Fact]
    public void a_loop_through_an_interface_names_it()
    {
        Assert.Equal(
            "An IReadOnlyDictionary loops in no fixed order, which can differ between machines; use OrderedMap or OrderedSet",
            MapCode.Message("IReadOnlyDictionary<int, int> view = new Dictionary<int, int>(); foreach (var pair in view) { }"));
    }
}
