using System.Globalization;
using System.Linq;
using Moonwell.Testing;

namespace Moonwell.Tests;

/// <summary>The invariant culture this assembly runs under (InvariantCulture) is what makes the cases that check text pass on a machine set to another format.</summary>
public class CultureTests
{
    // Cases that check text with a fraction or a minus sign, which a Swedish format writes as "2,5" and "−4".
    private static readonly string[] TextCases =
    {
        "text.plain",
        "text.nullable-from-int",
        "maths.vector2.text",
        "maths.raycast.ray-values",
        "heroes.skill-points",
    };

    [Fact]
    public void The_invariant_culture_makes_the_text_cases_pass()
    {
        Assert.Same(CultureInfo.InvariantCulture, CultureInfo.DefaultThreadCurrentCulture);
        Assert.Same(CultureInfo.InvariantCulture, CultureInfo.CurrentCulture);
        foreach (var name in TextCases)
        {
            Run(name);
        }

        var before = CultureInfo.CurrentCulture;
        CultureInfo.CurrentCulture = CultureInfo.GetCultureInfo("sv-SE");
        try
        {
            foreach (var name in TextCases)
            {
                Assert.Throws<ExpectationFailed>(() => Run(name));
            }
        }
        finally
        {
            CultureInfo.CurrentCulture = before;
        }
    }

    private static void Run(string name)
    {
        Cases.All().Single(c => c.Name == name).Run(FakeWorld.Install());
    }
}
