using System.Collections.Generic;
using static Moonwell.Natives;

namespace Desyncs
{
    public static class LuaMistakes
    {
        public static List<int> Numbers() => [1, 2];

        public static int Random() => GetRandomInt(1, 2);
    }
}
