CREATE TABLE notifications (
                                      server_uuid uuid NOT NULL,
                                      txt varchar NOT NULL,
                                      "status" int2 NOT NULL,
                                      "destination" int2 NOT NULL,
                                      server_timestamp timestamp NOT NULL,
                                      last_updated timestamp NOT NULL,
                                      CONSTRAINT notifications_pk PRIMARY KEY (server_uuid)
);

-- Column comments

COMMENT ON COLUMN public.notifications.server_uuid IS 'Primary key, generated server side';
COMMENT ON COLUMN public.notifications.txt IS 'Notification Text';
COMMENT ON COLUMN public.notifications."status" IS 'Status of the notification';
COMMENT ON COLUMN public.notifications."destination" IS 'Notification destination';
COMMENT ON COLUMN public.notifications.server_timestamp IS 'Timestamp generated by server';
COMMENT ON COLUMN public.notifications.last_updated IS 'Last updated timestamp';