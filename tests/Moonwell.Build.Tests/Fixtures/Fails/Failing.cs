using System;
using Moonwell;

namespace Fails
{
    public sealed class Throws : GameComponent
    {
        public override void OnGameStart()
        {
            throw new InvalidOperationException("thrown");
        }
    }

    public sealed class Holder
    {
        public int Value;
    }

    public sealed class ReadsNull : GameComponent
    {
        private readonly Holder? holder = null;

        public override void OnGameStart()
        {
            var value = holder!.Value;
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "value " + value);
        }
    }

    public sealed class CallsHelper : GameComponent
    {
        public override void OnGameStart()
        {
            Helper();
        }

        private static void Helper()
        {
            throw new InvalidOperationException("from helper");
        }
    }

    public sealed class TimerThrows : GameComponent
    {
        public override void OnGameStart()
        {
            Timer.After(0f, () =>
            {
                throw new InvalidOperationException("from timer");
            });
        }
    }

    public sealed class ReadsProperty : GameComponent
    {
        private int Broken => throw new InvalidOperationException("from property");

        public override void OnGameStart()
        {
            var value = Broken;
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "value " + value);
        }
    }

    public sealed class EveryZero : GameComponent
    {
        public override void OnGameStart()
        {
            Timer.Every(0f, () => { });
        }
    }
}
