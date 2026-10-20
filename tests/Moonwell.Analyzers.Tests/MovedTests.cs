namespace Moonwell.Analyzers.Tests;

/// <summary>MW0011: AttackType and DefenseType are core Moonwell's since 0.2.1, no longer enums of Moonwell.Objects.</summary>
public class MovedTests
{
    [Fact]
    public void a_name_of_the_old_enums_is_an_error_beside_the_compilers()
    {
        // Without using Moonwell, AttackType in a standard class is the property: AttackType.Magic does not compile (CS0176).
        Assert.Equal(new[]
        {
            "MW0011: AttackType",
            "MW0011: Moonwell.Objects.AttackType",
            "MW0011: Moonwell.Objects.DefenseType",
            "MW0011: O.AttackType",
            "MW0011: AttackType",
        }, MapCode.CheckBrokenFile("""
            using Moonwell.Objects;
            using O = Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public Mage()
                    {
                        AttackType = AttackType.Magic;
                    }
                }

                public static class Code
                {
                    public static int Run()
                    {
                        var attack = Moonwell.Objects.AttackType.Magic;
                        var defense = typeof(Moonwell.Objects.DefenseType);
                        var other = O.AttackType.Hero;
                        return (int)attack + (int)AttackType.Chaos;
                    }
                }
            }
            """));
    }

    [Fact]
    public void the_message_names_the_core_enum()
    {
        Assert.Equal(
            new[]
            {
                "Moonwell.Objects.DefenseType is Moonwell.DefenseType since Moonwell 0.2.1, which the damage package uses too: write Moonwell.DefenseType, or DefenseType with using Moonwell",
                "AttackType is Moonwell.AttackType since Moonwell 0.2.1, which the damage package uses too: write Moonwell.AttackType, or AttackType with using Moonwell",
            },
            MapCode.MessagesOfBrokenFile("""
                public static class Code
                {
                    public static int Run() => (int)Moonwell.Objects.DefenseType.Divine + (int)AttackType.Hero;
                }
                """));
    }

    [Fact]
    public void the_core_enums_other_choices_and_the_properties_of_their_names_are_accepted()
    {
        Assert.Empty(MapCode.CheckFile("""
            using Moonwell;
            using Moonwell.Objects;

            namespace Map
            {
                public class Mage : Human.Units.Footman
                {
                    public Mage()
                    {
                        AttackType = AttackType.Magic;
                        DefenseType = Moonwell.DefenseType.Divine;
                        Attack2AttackType = AttackType;
                        var shown = AttackType.ToString();
                    }

                    public static DefenseTypeInt Uprooted => DefenseTypeInt.Divine;
                }
            }
            """));
    }
}
