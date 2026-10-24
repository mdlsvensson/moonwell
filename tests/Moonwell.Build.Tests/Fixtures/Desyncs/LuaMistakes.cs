using static Moonwell.Natives;

namespace Desyncs
{
    public static class LuaMistakes
    {
        public static int Random() => GetRandomInt(1, 2);
    }
}
