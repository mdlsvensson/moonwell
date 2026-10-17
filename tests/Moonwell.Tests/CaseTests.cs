using System.Collections.Generic;
using System.Linq;
using Moonwell.Testing;

namespace Moonwell.Tests;

public class CaseTests
{
    public static IEnumerable<object[]> Names()
    {
        foreach (var testCase in Cases.All())
        {
            yield return new object[] { testCase.Name };
        }
    }

    [Theory]
    [MemberData(nameof(Names))]
    public void Runs(string name)
    {
        var testCase = Cases.All().Single(c => c.Name == name);
        testCase.Run(FakeWorld.Install());
    }
}
