using System.Reflection;
using Moonwell.Objects;

namespace Moonwell.Analyzers.Tests;

/// <summary>
/// MW0009 repeats the rules of <c>RuleChecks</c> in Moonwell.Objects, which the build runs. Each case here is compiled, then
/// run through the analyzer and constructed on .NET, and the analyzer's text must be the build's message, character for
/// character; a case the build allows must give nothing in either. A change to the wording or the rules in one place and
/// not the other fails here.
/// </summary>
public class FieldRulesParityTests
{
    /// <summary>A kind base written in the pattern of the generated ones, with fields that no generated class has: a range with no upper limit, a negative-only rule, a field without a label.</summary>
    private const string ProbeSource = """
        public abstract class Probe : Moonwell.Unit
        {
            private Declared? declared;

            protected Declared Fields => declared ??= new Declared(this);

            [Field("zzz1", Label = "At Least")]
            [Limits(1, double.PositiveInfinity)]
            public int AtLeast
            {
                get { return Fields.Get<int>(this, "zzz1"); }
                protected set { Fields.Put("zzz1", value); }
            }

            [Field("zzz2", Label = "Not Negative")]
            [NonNegative]
            public float NotNegative
            {
                get { return Fields.Get<float>(this, "zzz2"); }
                protected set { Fields.Put("zzz2", value); }
            }

            [Field("zzz3")]
            [Limits(2, 4)]
            public int Unlabelled
            {
                get { return Fields.Get<int>(this, "zzz3"); }
                protected set { Fields.Put("zzz3", value); }
            }
        }
        """;

    private const string Footman = "Moonwell.Objects.Human.Units.Footman";
    private const string StormBolt = "Moonwell.Objects.Human.Abilities.StormBolt";
    private const string Archmage = "Moonwell.Objects.Human.Units.Archmage";
    private const string Tree = "Moonwell.Objects.Doodads.AlchemySet";

    public static IEnumerable<object[]> Cases()
    {
        var suffix = new string('x', 51);
        var names = string.Join(",", Enumerable.Range(1, 33).Select(i => "N" + i));

        // base, attribute, statement; whether the build and the analyzer both allow it is part of the case.
        yield return new object[] { "range", Footman, "", "HitPoints = 600000;" };
        yield return new object[] { "range, lower end", Footman, "", "HitPoints = 0;" };
        yield return new object[] { "range, lifted", Footman, "[BeyondLimits(nameof(HitPoints))]", "HitPoints = 600000;" };
        yield return new object[] { "range, at the end", Footman, "", "HitPoints = 500000;" };
        yield return new object[] { "range, another property lifted", Footman, "[BeyondLimits(nameof(Speed))]", "HitPoints = 600000;" };
        yield return new object[] { "range of a field without a label", "Probe", "", "Unlabelled = 5;" };
        yield return new object[] { "range of a field without a label, in range", "Probe", "", "Unlabelled = 3;" };
        yield return new object[] { "or more", "Probe", "", "AtLeast = 0;" };
        yield return new object[] { "or more, lifted", "Probe", "[BeyondLimits(nameof(AtLeast))]", "AtLeast = 0;" };
        yield return new object[] { "at most", StormBolt, "", "RequirementsLevels = \"1,200\";" };
        yield return new object[] { "at most, lifted", StormBolt, "[BeyondLimits(nameof(RequirementsLevels))]", "RequirementsLevels = \"1,200\";" };
        yield return new object[] { "at most, in range", StormBolt, "", "RequirementsLevels = \"-5,100\";" };
        yield return new object[] { "negative", Footman, "", "HitPoints = -5;" };
        yield return new object[] { "negative, lifted", Footman, "[BeyondLimits(nameof(HitPoints))]", "HitPoints = -5;" };
        yield return new object[] { "negative the game forbids", Footman, "", "CollisionSize = -5f;" };
        yield return new object[] { "negative the game forbids, lifted", Footman, "[BeyondLimits(nameof(CollisionSize))]", "CollisionSize = -5f;" };
        yield return new object[] { "negative a standard object holds, lifted", Footman, "[BeyondLimits(nameof(ButtonPositionY))]", "ButtonPositionY = -11;" };
        yield return new object[] { "negative, no limits", "Probe", "", "NotNegative = -1f;" };
        yield return new object[] { "negative, no limits, lifted", "Probe", "[BeyondLimits(nameof(NotNegative))]", "NotNegative = -1f;" };
        yield return new object[] { "negative, no limits, zero", "Probe", "", "NotNegative = 0f;" };
        yield return new object[] { "level", StormBolt, "", "Cooldown = Levels(1f, 2f, 3900f);" };
        yield return new object[] { "level, lowest first", StormBolt, "", "Cooldown = Levels(1f, -1f, 3900f);" };
        yield return new object[] { "level, in range", StormBolt, "", "Cooldown = Levels(0f, 3600f);" };
        yield return new object[] { "set by id", Footman, "", "Set(\"uhpm\", 600000);" };
        yield return new object[] { "set by id and level", StormBolt, "", "Set(\"acdn\", 2, 3900f);" };
        yield return new object[] { "set by id and levels", StormBolt, "", "Set(\"acdn\", Levels(1f, 3900f));" };
        yield return new object[] { "empty name", Footman, "", "Name = \"\";" };
        yield return new object[] { "null name", Footman, "", "Name = null;" };
        yield return new object[] { "empty name, lifted", Footman, "[BeyondLimits(nameof(Name))]", "Name = \"\";" };
        yield return new object[] { "name", Footman, "", "Name = \"Giant\";" };
        yield return new object[] { "too long", Archmage, "", $"NameEditorSuffix = \"{suffix}\";" };
        yield return new object[] { "too long, lifted", Archmage, "[BeyondLimits(nameof(NameEditorSuffix))]", $"NameEditorSuffix = \"{suffix}\";" };
        yield return new object[] { "long enough", Archmage, "", $"NameEditorSuffix = \"{new string('x', 50)}\";" };
        yield return new object[] { "too many entries", Archmage, "", $"ProperNames = \"{names}\";" };
        yield return new object[] { "too many entries, lifted", Archmage, "[BeyondLimits(nameof(ProperNames))]", $"ProperNames = \"{names}\";" };
        yield return new object[] { "entries", Archmage, "", "ProperNames = \"A,B\"; ProperNames = \"\";" };
        yield return new object[] { "choice", Footman, "", "AttackType = (AttackType)42;" };
        yield return new object[] { "choice, lifted", Footman, "[BeyondLimits(nameof(AttackType))]", "AttackType = (AttackType)42;" };
        yield return new object[] { "choice, a member", Footman, "", "AttackType = AttackType.Normal;" };
        yield return new object[] { "int-stored choice", Footman, "", "DeathType = (DeathType)9;" };
        yield return new object[] { "flags choice", Footman, "", "PlacementRequires = (PathingListPrevent)(1 << 10);" };
        yield return new object[] { "flags choice, a combination", Footman, "", "PlacementRequires = PathingListPrevent.Unbuildable | PathingListPrevent.Unflyable;" };
        yield return new object[] { "float at the lower limit", Tree, "", "Scale = 0.01f;" };
        yield return new object[] { "float below the lower limit", Tree, "", "Scale = 0.0099999f;" };
        yield return new object[] { "float at the upper limit", Tree, "", "MaximumScale = 100f;" };
        yield return new object[] { "float above the upper limit", Tree, "", "MaximumScale = 100.00001f;" };
        yield return new object[] { "float that is not a number", Tree, "", "Scale = float.NaN;" };
        yield return new object[] { "float, an int constant", Tree, "", "Scale = 200;" };
    }

    [Theory]
    [MemberData(nameof(Cases))]
    public void the_analyzer_gives_the_text_the_build_gives(string name, string @base, string attribute, string statements)
    {
        var classes = (@base == "Probe" ? ProbeSource + "\n" : string.Empty)
            + FieldRulesTests.Class("Subject", @base, statements, attribute);
        var source = FieldRulesTests.File(classes);

        var analyzer = MapCode.MessagesOfFile(source);
        var thrown = Record.Exception(() => Construct(MapCode.Load(source)));

        if (thrown == null)
        {
            Assert.True(analyzer.Count == 0, name + ": the build allows it, the analyzer says " + string.Join(" | ", analyzer));
        }
        else
        {
            var rule = Assert.IsType<FieldRuleException>(thrown);
            Assert.Equal(new[] { rule.Message }, analyzer);
        }
    }

    [Fact]
    public void the_cases_cover_both_outcomes_and_every_wording()
    {
        var all = Cases().Select(c => (string)c[0]).ToList();
        foreach (var wording in new[] { "range", "or more", "at most", "negative", "level", "empty name", "too long", "too many entries", "choice", "flags choice", "float" })
        {
            Assert.Contains(all, name => name.Contains(wording));
        }
    }

    private static void Construct(Assembly assembly)
    {
        try
        {
            Activator.CreateInstance(assembly.GetType("Map.Subject")!);
        }
        catch (TargetInvocationException error) when (error.InnerException != null)
        {
            System.Runtime.ExceptionServices.ExceptionDispatchInfo.Capture(error.InnerException).Throw();
        }
    }
}
