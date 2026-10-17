using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases that check the harness itself.</summary>
    public static class HarnessCases
    {
        /// <summary>Handles made by new are distinct keys.</summary>
        public static void HandlesAreKeys(FakeWorld world)
        {
            var a = new unit();
            var b = new unit();
            var values = new Dictionary<unit, int>();
            values[a] = 1;
            values[b] = 2;
            Expect.Equal(1, values[a], "the first handle's value");
            Expect.Equal(2, values[b], "the second handle's value");
            Expect.True(a != b, "two handles are two objects");
        }

        /// <summary>A failed Expect check throws ExpectationFailed.</summary>
        public static void ExpectFails(FakeWorld world)
        {
            Expect.Throws<ExpectationFailed>(() => Expect.Equal(1, 2, "one"), "Expect.Equal of 1 and 2");
        }
    }
}
