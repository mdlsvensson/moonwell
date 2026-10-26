using System;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the input package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativeInputWorld : IInputWorld
    {
        /// <summary>Makes the three triggers of a player's mouse: its moves, its presses and its releases.</summary>
        public void ListenMouse(player p, Action moved, Action pressed, Action released)
        {
            var move = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(move, p, Natives.EVENT_PLAYER_MOUSE_MOVE);
            Natives.TriggerAddAction(move, moved);
            var press = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(press, p, Natives.EVENT_PLAYER_MOUSE_DOWN);
            Natives.TriggerAddAction(press, pressed);
            var release = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(release, p, Natives.EVENT_PLAYER_MOUSE_UP);
            Natives.TriggerAddAction(release, released);
        }

        /// <summary>The player of the event being delivered.</summary>
        public player EventPlayer() => Natives.GetTriggerPlayer();

        /// <summary>The x the event gave.</summary>
        public float EventMouseX() => Natives.BlzGetTriggerPlayerMouseX();

        /// <summary>The y the event gave.</summary>
        public float EventMouseY() => Natives.BlzGetTriggerPlayerMouseY();

        /// <summary>The event's button, compared with the game's three constants: the game's right button was seen with handle id 4, not common.j's 3, so its number is not used.</summary>
        public int EventMouseButton()
        {
            var button = Natives.BlzGetTriggerPlayerMouseButton();
            if (button == Natives.MOUSE_BUTTON_TYPE_LEFT)
            {
                return 1;
            }

            if (button == Natives.MOUSE_BUTTON_TYPE_MIDDLE)
            {
                return 2;
            }

            if (button == Natives.MOUSE_BUTTON_TYPE_RIGHT)
            {
                return 3;
            }

            return 0;
        }

        /// <summary>The world's top-right corner; GetWorldBounds makes a rect, which is read and left.</summary>
        public void WorldTopRight(out float maxX, out float maxY)
        {
            var bounds = Natives.GetWorldBounds();
            maxX = Natives.GetRectMaxX(bounds);
            maxY = Natives.GetRectMaxY(bounds);
        }

        /// <summary>The cursor's x on the screen of this machine.</summary>
        public int MouseScreenX() => Natives.BlzGetMouseScreenPosX();

        /// <summary>The cursor's y on the screen of this machine.</summary>
        public int MouseScreenY() => Natives.BlzGetMouseScreenPosY();

        /// <summary>The unit under the cursor on this machine; null when none.</summary>
        public unit? MouseFocusUnit() => Natives.BlzGetMouseFocusUnit();

        /// <summary>Whether a button (1 left, 2 middle, 3 right) is held on this machine now; false for any other number.</summary>
        public bool IsMouseButtonPressed(int button)
        {
            if (button == 1)
            {
                return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_LEFT);
            }

            if (button == 2)
            {
                return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_MIDDLE);
            }

            if (button == 3)
            {
                return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_RIGHT);
            }

            return false;
        }

        /// <summary>Moves a player's cursor on that player's machine only.</summary>
        public void SetMouseScreen(player p, int x, int y)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.BlzSetMousePos(x, y);
            }
        }

        /// <summary>Shows or hides a player's cursor on that player's machine only.</summary>
        public void ShowCursor(player p, bool show)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.BlzEnableCursor(show);
            }
        }

        /// <summary>The game's camera field of a Moonwell code: the named constant, never ConvertCameraField on the number.</summary>
        private static camerafield FieldOf(int field)
        {
            if (field == 0)
            {
                return Natives.CAMERA_FIELD_TARGET_DISTANCE;
            }

            if (field == 1)
            {
                return Natives.CAMERA_FIELD_FARZ;
            }

            if (field == 2)
            {
                return Natives.CAMERA_FIELD_ANGLE_OF_ATTACK;
            }

            if (field == 3)
            {
                return Natives.CAMERA_FIELD_FIELD_OF_VIEW;
            }

            if (field == 4)
            {
                return Natives.CAMERA_FIELD_ROLL;
            }

            if (field == 5)
            {
                return Natives.CAMERA_FIELD_ROTATION;
            }

            if (field == 6)
            {
                return Natives.CAMERA_FIELD_ZOFFSET;
            }

            if (field == 7)
            {
                return Natives.CAMERA_FIELD_NEARZ;
            }

            if (field == 8)
            {
                return Natives.CAMERA_FIELD_LOCAL_PITCH;
            }

            if (field == 9)
            {
                return Natives.CAMERA_FIELD_LOCAL_YAW;
            }

            if (field == 10)
            {
                return Natives.CAMERA_FIELD_LOCAL_ROLL;
            }

            if (field == 11)
            {
                return Natives.CAMERA_FIELD_DEPTH_OF_FIELD_DISTANCE;
            }

            if (field == 12)
            {
                return Natives.CAMERA_FIELD_DEPTH_OF_FIELD_SCALE;
            }

            if (field == 13)
            {
                return Natives.CAMERA_FIELD_ZABSOLUTE;
            }

            throw new ArgumentException("No camera field has the code " + field + ".");
        }

        /// <summary>One field of this machine's camera.</summary>
        public float CameraField(int field) => Natives.GetCameraField(FieldOf(field));

        /// <summary>The x this machine's camera looks at.</summary>
        public float CameraTargetX() => Natives.GetCameraTargetPositionX();

        /// <summary>The y this machine's camera looks at.</summary>
        public float CameraTargetY() => Natives.GetCameraTargetPositionY();

        /// <summary>The z this machine's camera looks at.</summary>
        public float CameraTargetZ() => Natives.GetCameraTargetPositionZ();

        /// <summary>The x of this machine's camera eye.</summary>
        public float CameraEyeX() => Natives.GetCameraEyePositionX();

        /// <summary>The y of this machine's camera eye.</summary>
        public float CameraEyeY() => Natives.GetCameraEyePositionY();

        /// <summary>The z of this machine's camera eye.</summary>
        public float CameraEyeZ() => Natives.GetCameraEyePositionZ();

        /// <summary>Changes a field of a player's camera on that player's machine only; the game takes the angles in degrees.</summary>
        public void SetCameraField(player p, int field, float value, float seconds)
        {
            var which = FieldOf(field);
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.SetCameraField(which, value, seconds);
            }
        }

        /// <summary>Pans a player's camera on that player's machine only.</summary>
        public void PanCamera(player p, float x, float y, float seconds)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.PanCameraToTimed(x, y, seconds);
            }
        }

        /// <summary>Locks a player's camera on a unit on that player's machine only, with no offset and the unit's facing not followed.</summary>
        public void FollowUnit(player p, unit u)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.SetCameraTargetController(u, 0f, 0f, false);
            }
        }

        /// <summary>Ends the lock by setting the camera's position to its own target, on that player's machine only.</summary>
        public void StopFollowing(player p)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.SetCameraPosition(Natives.GetCameraTargetPositionX(), Natives.GetCameraTargetPositionY());
            }
        }

        /// <summary>Puts a player's camera back to the game camera on that player's machine only.</summary>
        public void ResetCamera(player p, float seconds)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.ResetToGameCamera(seconds);
            }
        }

        /// <summary>Limits a player's camera on that player's machine only; the four corners go in the order World Editor's own script gives them.</summary>
        public void SetCameraBounds(player p, float minX, float minY, float maxX, float maxY)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.SetCameraBounds(minX, minY, minX, maxY, maxX, maxY, maxX, minY);
            }
        }

        /// <summary>Makes a setup as War3Net's generated script does: the fields in the order given, then the destination, then the camera type unless it is -1.</summary>
        public camerasetup MakeSetup(float[] values, int offset)
        {
            var setup = Natives.CreateCameraSetup();
            var count = (int)values[offset + 2];
            var at = offset + 3;
            for (var i = 0; i < count; i++)
            {
                Natives.CameraSetupSetField(setup, FieldOf((int)values[at]), values[at + 1], 0f);
                at += 2;
            }

            Natives.CameraSetupSetDestPosition(setup, values[offset], values[offset + 1], 0f);
            var type = (int)values[at];
            if (type != -1)
            {
                Natives.BlzCameraSetupSetCameraType(setup, type);
            }

            return setup;
        }

        /// <summary>Applies a setup to a player's camera on that player's machine only, taking the seconds given even if the setup says otherwise.</summary>
        public void ApplySetup(player p, camerasetup setup, float seconds)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.CameraSetupApplyForceDuration(setup, true, seconds);
            }
        }
    }
}
