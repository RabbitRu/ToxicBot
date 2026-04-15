create table response_log_old (
    "date" datetime not null
    ,"type" text not null check("type" in (
        'on_text'
        ,'on_sticker'
        ,'on_voice'
        ,'on_user_join'
        ,'on_user_left'
        ,'personal'
        ,'tagger'
        ,'on_photo'
    ))
    ,chat_id_hash blob not null
    ,user_id_hash blob not null
    ,extra json
);

insert into response_log_old select * from response_log where "type" != 'on_photo';

drop table response_log;

alter table response_log_old rename to response_log;

create index if not exists response_log_date_idx on response_log ("date");
